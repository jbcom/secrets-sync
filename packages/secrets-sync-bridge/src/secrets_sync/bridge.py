"""Python bridge for the SecretSync pipeline."""

from __future__ import annotations

import logging
import shutil
import subprocess

from importlib import import_module
from pathlib import Path
from typing import TYPE_CHECKING, Any, cast

from extended_data.containers import ExtendedDict, extend_data, to_builtin
from extended_data.io import DataFile
from extended_data.io.files import decode_file
from extended_data.primitives.formats.errors import DataDecodeError
from extended_data.primitives.redaction import redact_sensitive_text

from secrets_sync.models import (
    ConfigInfo,
    InfoLoggerLike,
    LifecycleLoggerLike,
    LoggerLike,
    RuntimeBackend,
    SyncOperation,
    SyncOptions,
    SyncResult,
)


if TYPE_CHECKING:
    from types import ModuleType


VALIDATE_RESULT_PARTS = 2
NATIVE_MODULE_NAME = "secrets_sync_native"


class SecretsSyncBridge:
    """Enterprise-grade SecretSync bridge.

    This bridge wraps the standalone SecretSync project (`jbcom/secrets-sync`)
    through the supported `secrets-sync` CLI, or through local gopy-generated
    bindings when the `secrets_sync_native` module is installed.

    Features:
    - Two-phase pipeline architecture (merge -> sync)
    - Secret inheritance and deep merging
    - AWS Organizations discovery
    - Dry-run with visual diff output
    - CI/CD integration with exit codes

    The published Python import stays `secrets_sync`; direct gopy output must
    use `secrets_sync_native` so generated bindings do not collide with the
    bridge package.
    """

    def __init__(
        self,
        cli_path: str | None = None,
        logger: LoggerLike | None = None,
        backend: RuntimeBackend | str = RuntimeBackend.AUTO,
    ) -> None:
        """Initialize the secrets-sync bridge.

        Args:
            cli_path: Path to secrets-sync CLI binary.
            logger: Standard logger or Extended Data lifecycle logger.
            backend: Runtime backend. `auto` prefers native gopy bindings when
                installed, then falls back to the CLI. `native` requires
                `secrets_sync_native`; `cli` only uses the CLI.
        """
        self.logger = logger or logging.getLogger("secrets_sync")
        self._backend = RuntimeBackend(backend)
        self._native_module = None if self._backend is RuntimeBackend.CLI else self._find_native_module()
        self._cli_path = cli_path or self._find_cli()
        self._log_info(f"SecretsSyncBridge initialized in {self.runtime_backend.value} mode")

    def _log_info(self, message: str) -> None:
        """Log an info message through a standard or Extended Data logger."""
        logger = self.logger
        if hasattr(logger, "info"):
            cast("InfoLoggerLike", logger).info(message)
            return
        cast("LifecycleLoggerLike", logger).logged_statement(message, log_level="info")

    def _find_cli(self) -> str | None:
        """Find the SecretSync `secrets-sync` CLI binary."""
        candidates = [
            "secrets-sync",
            "/usr/local/bin/secrets-sync",
            "/usr/bin/secrets-sync",
            str(Path.home() / "go" / "bin" / "secrets-sync"),
        ]

        for candidate in candidates:
            if shutil.which(candidate):
                return candidate

        return None

    def _find_native_module(self) -> ModuleType | None:
        """Find locally generated gopy bindings."""
        try:
            return import_module(NATIVE_MODULE_NAME)
        except ImportError:
            return None

    @property
    def cli_available(self) -> bool:
        """Return whether the CLI is available."""
        return self._cli_path is not None

    @property
    def native_available(self) -> bool:
        """Return whether direct gopy bindings are available."""
        return self._native_module is not None

    @property
    def runtime_backend(self) -> RuntimeBackend:
        """Return the backend that will handle pipeline execution."""
        if self._backend in {RuntimeBackend.NATIVE, RuntimeBackend.CLI}:
            return self._backend
        if self.native_available:
            return RuntimeBackend.NATIVE
        return RuntimeBackend.CLI

    def validate_config(self, config_path: str) -> ExtendedDict:
        """Validate a pipeline configuration file.

        Args:
            config_path: Path to YAML configuration file.

        Returns:
            Extended validation payload.
        """
        if self.runtime_backend is RuntimeBackend.NATIVE:
            is_valid, message = self._native_validate_config(config_path)
        else:
            is_valid, message = self._cli_validate_config(config_path)

        return extend_data(
            {
                "valid": is_valid,
                "message": message,
                "config_path": config_path,
            }
        )

    def _cli_validate_config(self, config_path: str) -> tuple[bool, str]:
        """Validate config via CLI."""
        if not self._cli_path:
            return False, "CLI not available"

        try:
            result = subprocess.run(  # noqa: S603 - executing the configured CLI is the bridge contract.
                [self._cli_path, "validate", "--config", config_path],
                capture_output=True,
                text=True,
                timeout=30,
                check=False,
            )
            if result.returncode == 0:
                return True, "Configuration is valid"
            return False, redact_sensitive_text(result.stderr or result.stdout)
        except subprocess.TimeoutExpired:
            return False, "Validation timed out"
        except Exception as e:
            return False, redact_sensitive_text(e)

    def _native_validate_config(self, config_path: str) -> tuple[bool, str]:
        """Validate config via gopy bindings."""
        native = self._require_native()
        try:
            result = native.ValidateConfig(config_path)
        except Exception as e:
            return False, redact_sensitive_text(e)

        if isinstance(result, tuple | list) and len(result) >= VALIDATE_RESULT_PARTS:
            return bool(result[0]), str(redact_sensitive_text(result[1]))
        return bool(result), "Configuration is valid" if result else "Configuration is invalid"

    def get_config_info(self, config_path: str) -> ExtendedDict:
        """Get detailed information about a configuration.

        Args:
            config_path: Path to YAML configuration file.

        Returns:
            Extended configuration details payload.
        """
        return self._cli_get_config_info(config_path).to_dict()

    def _cli_get_config_info(self, config_path: str) -> ConfigInfo:
        """Get config info via CLI."""
        try:
            cfg = to_builtin(DataFile.read(config_path, suffix="yaml", as_extended=True).data)

            if not isinstance(cfg, dict):
                cfg = {}
            sources = cfg.get("sources", {})
            if not isinstance(sources, dict):
                sources = {}
            targets = cfg.get("targets", {})
            if not isinstance(targets, dict):
                targets = {}
            vault = cfg.get("vault", {})
            if not isinstance(vault, dict):
                vault = {}
            aws = cfg.get("aws", {})
            if not isinstance(aws, dict):
                aws = {}

            return ConfigInfo(
                valid=True,
                source_count=len(sources),
                target_count=len(targets),
                sources=list(sources.keys()),
                targets=list(targets.keys()),
                has_merge_store="merge_store" in cfg,
                vault_address=vault.get("address", ""),
                aws_region=aws.get("region", ""),
            )
        except FileNotFoundError:
            return ConfigInfo(error_message=f"Configuration file not found: {redact_sensitive_text(config_path)}")
        except DataDecodeError as e:
            return ConfigInfo(error_message=f"Error parsing YAML file: {redact_sensitive_text(e)}")

    def run_pipeline(
        self,
        config_path: str,
        options: SyncOptions | None = None,
    ) -> ExtendedDict:
        """Execute the secrets synchronization pipeline.

        Args:
            config_path: Path to YAML configuration file.
            options: Execution options. Defaults to full pipeline.

        Returns:
            Extended sync result payload.
        """
        options = options or SyncOptions()

        if self.runtime_backend is RuntimeBackend.NATIVE:
            return self._native_run_pipeline(config_path, options).to_dict()
        return self._cli_run_pipeline(config_path, options).to_dict()

    def _build_pipeline_command(self, config_path: str, options: SyncOptions) -> list[str]:
        """Build the supported CLI pipeline command."""
        if self._cli_path is None:
            return []

        cmd = [
            self._cli_path,
            "pipeline",
            "--config",
            config_path,
            "--output",
            "json",
        ]

        if options.operation == SyncOperation.MERGE:
            cmd.append("--merge-only")
        elif options.operation == SyncOperation.SYNC:
            cmd.append("--sync-only")

        if options.dry_run:
            cmd.append("--dry-run")
        if options.compute_diff:
            cmd.append("--diff")
        if options.targets:
            cmd.extend(["--targets", ",".join(options.targets)])
        cmd.append(f"--continue-on-error={str(options.continue_on_error).lower()}")
        if options.parallelism > 0:
            cmd.extend(["--parallelism", str(options.parallelism)])

        return cmd

    def _parse_pipeline_process(self, result: subprocess.CompletedProcess[str]) -> SyncResult:
        """Parse a completed CLI process into a sync result."""
        stdout = result.stdout.strip()
        if not stdout:
            if result.returncode == 0:
                return SyncResult(
                    success=False,
                    error_message="secrets-sync produced no JSON output",
                )
            return SyncResult(
                success=False,
                error_message=redact_sensitive_text(result.stderr or result.stdout),
            )

        try:
            output = to_builtin(decode_file(stdout, suffix="json", as_extended=True))
        except DataDecodeError as e:
            if result.returncode == 0:
                return SyncResult(
                    success=False,
                    error_message=f"Failed to parse output: {redact_sensitive_text(e)}",
                )
            return SyncResult(
                success=False,
                error_message=redact_sensitive_text(result.stderr or result.stdout),
            )

        if not isinstance(output, dict) or "success" not in output:
            return SyncResult(
                success=False,
                error_message=(
                    "Unsupported secrets-sync JSON output: expected pipeline result envelope. "
                    "Upgrade secrets-sync to a version that emits the stable result envelope."
                ),
            )

        parsed = SyncResult.from_cli_output(output)
        if result.returncode != 0 and not parsed.error_message:
            parsed.error_message = redact_sensitive_text(
                result.stderr or f"secrets-sync exited with status {result.returncode}"
            )
        return parsed

    def _cli_run_pipeline(
        self,
        config_path: str,
        options: SyncOptions,
    ) -> SyncResult:
        """Run pipeline via CLI."""
        if not self._cli_path:
            return SyncResult(
                success=False,
                error_message="secrets-sync CLI not available",
            )

        cmd = self._build_pipeline_command(config_path, options)

        try:
            result = subprocess.run(  # noqa: S603 - executing the configured CLI is the bridge contract.
                cmd,
                capture_output=True,
                text=True,
                timeout=600,
                check=False,
            )
            return self._parse_pipeline_process(result)
        except subprocess.TimeoutExpired:
            return SyncResult(
                success=False,
                error_message="Pipeline execution timed out",
            )
        except Exception as e:
            return SyncResult(
                success=False,
                error_message=redact_sensitive_text(e),
            )

    def _require_native(self) -> ModuleType:
        """Return the native module or raise a clear runtime error."""
        if self._native_module is None:
            raise RuntimeError(
                "secrets_sync_native is not installed; run `make python-bindings` "
                "and install the generated package, or use backend='cli'."
            )
        return self._native_module

    def _native_options(self, native: ModuleType, options: SyncOptions) -> Any:
        """Build gopy SyncOptions while tolerating generated wrapper differences."""
        if hasattr(native, "DefaultSyncOptions"):
            native_options = native.DefaultSyncOptions()
        elif hasattr(native, "SyncOptions"):
            native_options = native.SyncOptions()
        else:
            native_options = type("NativeSyncOptions", (), {})()

        self._set_native_attr(native_options, "DryRun", "dry_run", value=options.dry_run)
        self._set_native_attr(native_options, "Operation", "operation", value=options.operation.value)
        self._set_native_attr(native_options, "Targets", "targets", value=",".join(options.targets))
        self._set_native_attr(
            native_options,
            "ContinueOnError",
            "continue_on_error",
            value=options.continue_on_error,
        )
        self._set_native_attr(native_options, "Parallelism", "parallelism", value=options.parallelism)
        self._set_native_attr(native_options, "ComputeDiff", "compute_diff", value=options.compute_diff)
        self._set_native_attr(native_options, "OutputFormat", "output_format", value=options.output_format.value)
        self._set_native_attr(native_options, "ShowValues", "show_values", value=False)
        return native_options

    @staticmethod
    def _set_native_attr(target: Any, *names: str, value: Any) -> None:
        """Set a gopy option field without assuming exact generated casing."""
        for name in names:
            if hasattr(target, name):
                setattr(target, name, value)
                return
        setattr(target, names[0], value)

    def _native_run_pipeline(
        self,
        config_path: str,
        options: SyncOptions,
    ) -> SyncResult:
        """Run pipeline via gopy bindings."""
        try:
            native = self._require_native()
            native_options = self._native_options(native, options)
            return SyncResult.from_native_output(native.RunPipeline(config_path, native_options))
        except Exception as e:
            return SyncResult(
                success=False,
                error_message=redact_sensitive_text(e),
            )

    def dry_run(self, config_path: str) -> ExtendedDict:
        """Perform a dry run of the pipeline.

        Args:
            config_path: Path to YAML configuration file.

        Returns:
            Extended dry-run result payload.
        """
        options = SyncOptions(dry_run=True, compute_diff=True)
        return self.run_pipeline(config_path, options)

    def merge(self, config_path: str, dry_run: bool = False) -> ExtendedDict:
        """Run only the merge phase of the pipeline.

        Args:
            config_path: Path to YAML configuration file.
            dry_run: If True, do not make actual changes.

        Returns:
            Extended merge result payload.
        """
        options = SyncOptions(
            operation=SyncOperation.MERGE,
            dry_run=dry_run,
            compute_diff=dry_run,
        )
        return self.run_pipeline(config_path, options)

    def sync(self, config_path: str, dry_run: bool = False) -> ExtendedDict:
        """Run only the sync phase of the pipeline.

        Args:
            config_path: Path to YAML configuration file.
            dry_run: If True, do not make actual changes.

        Returns:
            Extended sync result payload.
        """
        options = SyncOptions(
            operation=SyncOperation.SYNC,
            dry_run=dry_run,
            compute_diff=dry_run,
        )
        return self.run_pipeline(config_path, options)

    def get_targets(self, config_path: str) -> ExtendedDict:
        """Get the list of targets from a configuration.

        Args:
            config_path: Path to YAML configuration file.

        Returns:
            Extended targets payload.
        """
        info = self.get_config_info(config_path)
        targets = info.get("targets", [])
        return extend_data(
            {
                "targets": targets,
                "count": len(targets),
                "error_message": info.get("error_message", ""),
            }
        )

    def get_sources(self, config_path: str) -> ExtendedDict:
        """Get the list of sources from a configuration.

        Args:
            config_path: Path to YAML configuration file.

        Returns:
            Extended sources payload.
        """
        info = self.get_config_info(config_path)
        sources = info.get("sources", [])
        return extend_data(
            {
                "sources": sources,
                "count": len(sources),
                "error_message": info.get("error_message", ""),
            }
        )
