"""Data models and protocols for the SecretSync Python bridge."""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
from enum import StrEnum
from typing import Any, Protocol, TypeAlias

from extended_data.containers import ExtendedDict, extend_data
from extended_data.io import wrap_raw_data_for_export
from extended_data.primitives.redaction import redact_sensitive_data, redact_sensitive_text


class InfoLoggerLike(Protocol):
    """Standard logger protocol accepted by the bridge."""

    def info(self, message: str, *args: Any, **kwargs: Any) -> Any:
        """Log an informational message."""


class LifecycleLoggerLike(Protocol):
    """Extended Data lifecycle logger protocol accepted by the bridge."""

    def logged_statement(self, msg: str, **kwargs: Any) -> Any:
        """Log a lifecycle statement."""


LoggerLike: TypeAlias = InfoLoggerLike | LifecycleLoggerLike


class SyncOperation(StrEnum):
    """Pipeline operation types."""

    MERGE = "merge"
    SYNC = "sync"
    PIPELINE = "pipeline"


class OutputFormat(StrEnum):
    """Output format for diff display."""

    HUMAN = "human"
    JSON = "json"
    GITHUB = "github"
    COMPACT = "compact"
    SIDE_BY_SIDE = "side-by-side"


class RuntimeBackend(StrEnum):
    """SecretSync runtime backend selection."""

    AUTO = "auto"
    CLI = "cli"
    NATIVE = "native"


@dataclass
class SyncOptions:
    """Options for pipeline execution."""

    dry_run: bool = False
    operation: SyncOperation = SyncOperation.PIPELINE
    targets: list[str] = field(default_factory=list)
    continue_on_error: bool = True
    parallelism: int = 0
    compute_diff: bool = False
    output_format: OutputFormat = OutputFormat.JSON


@dataclass
class SyncResult:
    """Result of a sync operation."""

    success: bool = False
    target_count: int = 0
    secrets_processed: int = 0
    secrets_added: int = 0
    secrets_modified: int = 0
    secrets_removed: int = 0
    secrets_unchanged: int = 0
    duration_ms: int = 0
    error_message: str = ""
    results_json: str = ""
    diff_output: str = ""

    @classmethod
    def from_cli_output(cls, output: dict[str, Any]) -> SyncResult:
        """Create from CLI JSON output."""
        safe_output = redact_sensitive_data(output)
        return cls(
            success=safe_output.get("success", False),
            target_count=safe_output.get("target_count", 0),
            secrets_processed=safe_output.get("secrets_processed", 0),
            secrets_added=safe_output.get("secrets_added", 0),
            secrets_modified=safe_output.get("secrets_modified", 0),
            secrets_removed=safe_output.get("secrets_removed", 0),
            secrets_unchanged=safe_output.get("secrets_unchanged", 0),
            duration_ms=safe_output.get("duration_ms", 0),
            error_message=safe_output.get("error_message", ""),
            results_json=wrap_raw_data_for_export(
                safe_output.get("results", []),
                allow_encoding="json",
                indent_2=True,
            ),
            diff_output=safe_output.get("diff_output", ""),
        )

    @classmethod
    def from_native_output(cls, output: Any) -> SyncResult:
        """Create from a gopy native result object or mapping."""
        safe_output = redact_sensitive_data(_native_result_to_mapping(output))
        results = safe_output.get("results_json", "")
        if not results and "results" in safe_output:
            results = wrap_raw_data_for_export(
                safe_output.get("results", []),
                allow_encoding="json",
                indent_2=True,
            )
        elif isinstance(results, str):
            results = redact_sensitive_text(results)

        diff_output = safe_output.get("diff_output", "")
        if isinstance(diff_output, str):
            diff_output = redact_sensitive_text(diff_output)

        error_message = safe_output.get("error_message", "")
        if isinstance(error_message, str):
            error_message = redact_sensitive_text(error_message)

        return cls(
            success=bool(safe_output.get("success", False)),
            target_count=int(safe_output.get("target_count", 0) or 0),
            secrets_processed=int(safe_output.get("secrets_processed", 0) or 0),
            secrets_added=int(safe_output.get("secrets_added", 0) or 0),
            secrets_modified=int(safe_output.get("secrets_modified", 0) or 0),
            secrets_removed=int(safe_output.get("secrets_removed", 0) or 0),
            secrets_unchanged=int(safe_output.get("secrets_unchanged", 0) or 0),
            duration_ms=int(safe_output.get("duration_ms", 0) or 0),
            error_message=str(error_message),
            results_json=str(results),
            diff_output=str(diff_output),
        )

    def to_dict(self) -> ExtendedDict:
        """Return an extended sync result payload."""
        return extend_data(asdict(self))


@dataclass
class ConfigInfo:
    """Information about a pipeline configuration."""

    valid: bool = False
    error_message: str = ""
    source_count: int = 0
    target_count: int = 0
    sources: list[str] = field(default_factory=list)
    targets: list[str] = field(default_factory=list)
    has_merge_store: bool = False
    vault_address: str = ""
    aws_region: str = ""

    def to_dict(self) -> ExtendedDict:
        """Return an extended config info payload."""
        return extend_data(asdict(self))


def _native_result_to_mapping(output: Any) -> dict[str, Any]:
    """Normalize gopy result structs and dicts into bridge field names."""
    if isinstance(output, SyncResult):
        return asdict(output)
    if isinstance(output, dict):
        return {_camel_or_snake_to_snake(str(key)): value for key, value in output.items()}

    fields = {
        "success": ("Success", "success"),
        "target_count": ("TargetCount", "target_count"),
        "secrets_processed": ("SecretsProcessed", "secrets_processed"),
        "secrets_added": ("SecretsAdded", "secrets_added"),
        "secrets_modified": ("SecretsModified", "secrets_modified"),
        "secrets_removed": ("SecretsRemoved", "secrets_removed"),
        "secrets_unchanged": ("SecretsUnchanged", "secrets_unchanged"),
        "duration_ms": ("DurationMs", "duration_ms"),
        "error_message": ("ErrorMessage", "error_message"),
        "results_json": ("ResultsJSON", "results_json"),
        "diff_output": ("DiffOutput", "diff_output"),
    }
    normalized: dict[str, Any] = {}
    for field_name, names in fields.items():
        for name in names:
            if hasattr(output, name):
                normalized[field_name] = getattr(output, name)
                break
    return normalized


def _camel_or_snake_to_snake(value: str) -> str:
    """Convert exported Go field names to Python result field names."""
    replacements = {
        "Success": "success",
        "TargetCount": "target_count",
        "SecretsProcessed": "secrets_processed",
        "SecretsAdded": "secrets_added",
        "SecretsModified": "secrets_modified",
        "SecretsRemoved": "secrets_removed",
        "SecretsUnchanged": "secrets_unchanged",
        "DurationMs": "duration_ms",
        "ErrorMessage": "error_message",
        "ResultsJSON": "results_json",
        "DiffOutput": "diff_output",
    }
    return replacements.get(value, value)
