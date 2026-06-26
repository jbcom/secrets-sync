import json
import tomllib

from importlib import util
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import MagicMock, patch

import pytest
import yaml

from extended_data.containers import ExtendedDict, ExtendedList, ExtendedString
from extended_data.io import DataFile
from extended_data.primitives.formats.errors import DataDecodeError

import secrets_sync.bridge as bridge_module
import secrets_sync.models as models_module

from secrets_sync import (
    OutputFormat,
    RuntimeBackend,
    SecretsSyncBridge,
    SyncOperation,
    SyncOptions,
    SyncResult,
)


@pytest.fixture
def mock_logger() -> MagicMock:
    return MagicMock()


@pytest.fixture
def bridge(mock_logger: MagicMock) -> SecretsSyncBridge:
    return SecretsSyncBridge(cli_path="/usr/bin/secrets-sync", logger=mock_logger)


def test_secrets_bridge_supports_cli_and_native_runtime_contracts() -> None:
    """SecretSync should expose one bridge API over CLI and gopy runtimes."""
    cli_bridge = SecretsSyncBridge(cli_path="/usr/bin/secrets-sync", backend=RuntimeBackend.CLI)

    assert cli_bridge.runtime_backend is RuntimeBackend.CLI
    assert cli_bridge.cli_available is True
    assert cli_bridge.native_available is False


def test_bridge_package_has_no_agent_framework_surface() -> None:
    """Agent-framework adapters belong in agentic-crew, not secrets-sync."""
    pyproject_path = Path(__file__).resolve().parents[1] / "pyproject.toml"
    pyproject = tomllib.loads(pyproject_path.read_text())
    optional_dependencies = pyproject["project"].get("optional-dependencies", {})

    assert util.find_spec("secrets_sync.tools") is None
    assert "pydantic" not in pyproject["project"]["dependencies"]
    assert {"langchain", "crewai", "strands", "all"}.isdisjoint(optional_dependencies)


def test_auto_runtime_falls_back_to_cli_when_native_module_is_absent() -> None:
    with patch("secrets_sync.bridge.import_module", side_effect=ImportError):
        auto_bridge = SecretsSyncBridge(cli_path="/usr/bin/secrets-sync")

    assert auto_bridge.runtime_backend is RuntimeBackend.CLI
    assert auto_bridge.native_available is False


def test_native_runtime_uses_gopy_module_when_available() -> None:
    calls: dict[str, object] = {}

    class NativeOptions:
        DryRun = False
        Operation = ""
        Targets = ""
        ContinueOnError = False
        Parallelism = 0
        ComputeDiff = False
        OutputFormat = ""
        ShowValues = True

    def default_sync_options() -> NativeOptions:
        return NativeOptions()

    def validate_config(config_path: str) -> tuple[bool, str]:
        calls["validated_path"] = config_path
        return True, "Configuration is valid"

    def run_pipeline(config_path: str, options: NativeOptions) -> SimpleNamespace:
        calls["pipeline_path"] = config_path
        calls["options"] = options
        return SimpleNamespace(
            Success=True,
            SecretsProcessed=7,
            ResultsJSON='[{"target":"prod"}]',
            DiffOutput="",
        )

    native_module = SimpleNamespace(
        DefaultSyncOptions=default_sync_options,
        ValidateConfig=validate_config,
        RunPipeline=run_pipeline,
    )

    with patch("secrets_sync.bridge.import_module", return_value=native_module):
        native_bridge = SecretsSyncBridge(backend=RuntimeBackend.NATIVE)

    validation = native_bridge.validate_config("pipeline.yaml")
    result = native_bridge.run_pipeline(
        "pipeline.yaml",
        SyncOptions(
            dry_run=True,
            operation=SyncOperation.SYNC,
            targets=["prod", "staging"],
            parallelism=3,
            compute_diff=True,
        ),
    )

    assert native_bridge.runtime_backend is RuntimeBackend.NATIVE
    assert native_bridge.native_available is True
    assert validation["valid"] is True
    assert calls["validated_path"] == "pipeline.yaml"
    assert result["success"] is True
    assert result["secrets_processed"] == 7
    options = calls["options"]
    assert isinstance(options, NativeOptions)
    assert options.DryRun is True
    assert options.Operation == "sync"
    assert options.Targets == "prod,staging"
    assert options.Parallelism == 3
    assert options.ComputeDiff is True


def test_cli_get_config_info_valid(bridge: SecretsSyncBridge, tmp_path: Path) -> None:
    config_file = tmp_path / "config.yaml"
    config_data = {
        "sources": {"src1": {}, "src2": {}},
        "targets": {"tgt1": {}},
        "vault": {"address": "http://vault:8200"},
        "aws": {"region": "us-east-1"},
        "merge_store": {},
    }
    config_file.write_text(yaml.dump(config_data))

    info = bridge.get_config_info(str(config_file))

    assert isinstance(info, ExtendedDict)
    assert isinstance(info["sources"], ExtendedList)
    assert info["valid"] is True
    assert info["source_count"] == 2
    assert info["target_count"] == 1
    assert "src1" in info["sources"]
    assert "src2" in info["sources"]
    assert "tgt1" in info["targets"]
    assert info["has_merge_store"] is True
    assert info["vault_address"] == "http://vault:8200"
    assert info["aws_region"] == "us-east-1"


@patch("secrets_sync.bridge.DataFile.read")
def test_cli_get_config_info_reads_through_data_file(
    mock_read: MagicMock,
    bridge: SecretsSyncBridge,
) -> None:
    mock_read.return_value = DataFile.decode(
        "sources:\n  src1: {}\ntargets:\n  tgt1: {}\n",
        file_path="config.yaml",
        suffix="yaml",
    )

    info = bridge.get_config_info("config.yaml")

    mock_read.assert_called_once_with("config.yaml", suffix="yaml", as_extended=True)
    assert info["valid"] is True
    assert info["source_count"] == 1
    assert info["target_count"] == 1
    assert info["sources"] == ["src1"]
    assert info["targets"] == ["tgt1"]


def test_cli_get_config_info_not_found(bridge: SecretsSyncBridge) -> None:
    info = bridge.get_config_info("/non/existent/path.yaml")
    assert isinstance(info, ExtendedDict)
    assert info["valid"] is False
    assert "Configuration file not found" in info["error_message"]


def test_cli_get_config_info_invalid_yaml(bridge: SecretsSyncBridge, tmp_path: Path) -> None:
    config_file = tmp_path / "config.yaml"
    config_file.write_text("invalid: yaml: :")

    info = bridge.get_config_info(str(config_file))
    assert isinstance(info, ExtendedDict)
    assert info["valid"] is False
    assert "Error parsing YAML file" in info["error_message"]


def test_cli_get_config_info_empty_file(bridge: SecretsSyncBridge, tmp_path: Path) -> None:
    config_file = tmp_path / "config.yaml"
    config_file.write_text("")

    info = bridge.get_config_info(str(config_file))
    assert isinstance(info, ExtendedDict)
    assert info["valid"] is True
    assert info["source_count"] == 0


def test_cli_get_targets_and_sources_return_extended_payloads(bridge: SecretsSyncBridge, tmp_path: Path) -> None:
    config_file = tmp_path / "config.yaml"
    config_file.write_text(
        yaml.dump(
            {
                "sources": {"vault/prod": {}, "vault/dev": {}},
                "targets": {"prod": {}, "dev": {}},
            }
        )
    )

    targets = bridge.get_targets(str(config_file))
    sources = bridge.get_sources(str(config_file))

    assert isinstance(targets, ExtendedDict)
    assert isinstance(targets["targets"], ExtendedList)
    assert isinstance(targets["targets"][0], ExtendedString)
    assert targets["count"] == 2
    assert set(targets["targets"]) == {"prod", "dev"}
    assert isinstance(sources, ExtendedDict)
    assert isinstance(sources["sources"], ExtendedList)
    assert set(sources["sources"]) == {"vault/prod", "vault/dev"}


@patch("subprocess.run")
def test_cli_run_pipeline_operation(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout=json.dumps({"success": True, "secrets_processed": 5}),
        stderr="",
    )

    options = SyncOptions(operation=SyncOperation.MERGE)
    result = bridge.run_pipeline("config.yaml", options)

    assert isinstance(result, ExtendedDict)
    assert result["success"] is True
    assert result["secrets_processed"] == 5

    # Check that it uses "pipeline" command with "--merge-only" flag
    args = mock_run.call_args[0][0]
    assert args[1] == "pipeline"
    assert "--merge-only" in args
    assert args.count("--output") == 1
    assert args[args.index("--output") + 1] == "json"


@patch("subprocess.run")
def test_cli_run_pipeline_diff_and_format(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout=json.dumps({"success": True, "diff_output": "some diff"}),
        stderr="",
    )

    options = SyncOptions(
        compute_diff=True,
        output_format=OutputFormat.JSON,
    )
    result = bridge.run_pipeline("config.yaml", options)

    assert isinstance(result, ExtendedDict)
    assert result["success"] is True

    args = mock_run.call_args[0][0]
    assert "--diff" in args
    assert args.count("--output") == 1
    assert args[args.index("--output") + 1] == "json"


@patch("subprocess.run")
def test_cli_run_pipeline_default_output_is_json(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout=json.dumps({"success": True}),
        stderr="",
    )

    result = bridge.run_pipeline("config.yaml")

    assert isinstance(result, ExtendedDict)
    assert result["success"] is True
    args = mock_run.call_args[0][0]
    assert args.count("--output") == 1
    assert args[args.index("--output") + 1] == "json"
    assert "--parallelism" not in args
    assert "--continue-on-error=true" in args


@patch("subprocess.run")
def test_cli_run_pipeline_parses_result_envelope(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    output = {
        "success": True,
        "target_count": 2,
        "secrets_processed": 5,
        "secrets_added": 1,
        "secrets_modified": 2,
        "secrets_removed": 0,
        "secrets_unchanged": 2,
        "duration_ms": 321,
        "results": [
            {"target": "prod", "phase": "merge", "success": True},
            {"target": "prod", "phase": "sync", "success": True},
        ],
        "diff_output": '{"summary":{"added":1}}',
        "diff": {"dry_run": True},
    }
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout=json.dumps(output),
        stderr="",
    )

    result = bridge.run_pipeline("config.yaml")

    assert isinstance(result, ExtendedDict)
    assert result["success"] is True
    assert result["target_count"] == 2
    assert result["secrets_processed"] == 5
    assert result["secrets_added"] == 1
    assert result["secrets_modified"] == 2
    assert result["secrets_unchanged"] == 2
    assert result["duration_ms"] == 321
    assert json.loads(str(result["results_json"])) == output["results"]
    assert result["diff_output"] == '{"summary":{"added":1}}'


@patch("secrets_sync.bridge.decode_file", wraps=bridge_module.decode_file)
@patch("subprocess.run")
def test_cli_run_pipeline_decodes_result_envelope_through_data_boundary(
    mock_run: MagicMock,
    mock_decode_file: MagicMock,
    bridge: SecretsSyncBridge,
) -> None:
    """SecretSync JSON envelopes should use shared data decoding, not local json.loads."""
    stdout = json.dumps({"success": True, "results": [{"target": "prod"}]})
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout=stdout,
        stderr="",
    )

    result = bridge.run_pipeline("config.yaml")

    assert result["success"] is True
    assert '"target": "prod"' in result["results_json"]
    mock_decode_file.assert_called_once_with(stdout, suffix="json", as_extended=True)


def test_sync_result_results_json_uses_shared_export_boundary() -> None:
    """SecretSync result details should serialize through the shared export boundary."""
    output = {"success": True, "results": [{"target": "prod"}]}

    with patch(
        "secrets_sync.models.wrap_raw_data_for_export",
        wraps=models_module.wrap_raw_data_for_export,
    ) as mock_wrap_for_export:
        result = SyncResult.from_cli_output(output)

    assert '"target": "prod"' in result.results_json
    mock_wrap_for_export.assert_called_once_with(output["results"], allow_encoding="json", indent_2=True)


@patch("subprocess.run")
def test_cli_run_pipeline_rejects_legacy_raw_diff_json(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout=json.dumps(
            {
                "dry_run": True,
                "summary": {"added": 1, "modified": 0, "removed": 0, "unchanged": 0},
                "targets": [],
            }
        ),
        stderr="",
    )

    result = bridge.run_pipeline("config.yaml", SyncOptions(dry_run=True, compute_diff=True))

    assert isinstance(result, ExtendedDict)
    assert result["success"] is False
    assert "expected pipeline result envelope" in result["error_message"]
    assert "native bindings" not in result["error_message"]


@patch("subprocess.run")
def test_cli_run_pipeline_parses_failure_result_envelope(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=1,
        stdout=json.dumps(
            {
                "success": False,
                "target_count": 1,
                "secrets_processed": 2,
                "error_message": "pipeline completed with errors",
                "results": [{"target": "prod", "phase": "sync", "success": False, "error": "denied"}],
            }
        ),
        stderr="Error: pipeline completed with errors\n",
    )

    result = bridge.run_pipeline("config.yaml")

    assert isinstance(result, ExtendedDict)
    assert result["success"] is False
    assert result["target_count"] == 1
    assert result["secrets_processed"] == 2
    assert result["error_message"] == "pipeline completed with errors"
    assert json.loads(str(result["results_json"]))[0]["error"] == "denied"


@patch("subprocess.run")
def test_cli_run_pipeline_redacts_failure_result_envelope(
    mock_run: MagicMock,
    bridge: SecretsSyncBridge,
) -> None:
    mock_run.return_value = MagicMock(
        returncode=1,
        stdout=json.dumps(
            {
                "success": False,
                "error_message": "pipeline failed password=hunter2 Authorization: Bearer raw_token",
                "results": [
                    {
                        "target": "prod",
                        "success": False,
                        "error": "target denied api_key=key_123",
                        "password": "hunter2",
                    }
                ],
                "diff_output": "changed token=tok_123",
            }
        ),
        stderr="",
    )

    result = bridge.run_pipeline("config.yaml")

    assert result["success"] is False
    assert "hunter2" not in result["error_message"]
    assert "raw_token" not in result["error_message"]
    assert "[REDACTED]" in result["error_message"]
    assert "hunter2" not in result["results_json"]
    assert "key_123" not in result["results_json"]
    assert '"password": "[REDACTED]"' in result["results_json"]
    assert "tok_123" not in result["diff_output"]
    assert "[REDACTED]" in result["diff_output"]


@patch("subprocess.run")
def test_cli_run_pipeline_failure_envelope_uses_stderr_when_error_message_missing(
    mock_run: MagicMock,
    bridge: SecretsSyncBridge,
) -> None:
    mock_run.return_value = MagicMock(
        returncode=1,
        stdout=json.dumps({"success": False, "results": []}),
        stderr="Error: boom\n",
    )

    result = bridge.run_pipeline("config.yaml")

    assert isinstance(result, ExtendedDict)
    assert result["success"] is False
    assert result["error_message"] == "Error: boom\n"


@patch("subprocess.run")
def test_cli_run_pipeline_success_without_json_is_error(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout="",
        stderr="",
    )

    result = bridge.run_pipeline("config.yaml")

    assert isinstance(result, ExtendedDict)
    assert result["success"] is False
    assert "produced no JSON output" in result["error_message"]


@patch("secrets_sync.bridge.decode_file")
@patch("subprocess.run")
def test_cli_run_pipeline_success_parse_error_is_redacted(
    mock_run: MagicMock,
    mock_decode_file: MagicMock,
    bridge: SecretsSyncBridge,
) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout="not json",
        stderr="",
    )
    mock_decode_file.side_effect = DataDecodeError(
        "JSON",
        reason="invalid password=hunter2 Authorization: Bearer raw_token",
    )

    result = bridge.run_pipeline("config.yaml")

    assert result["success"] is False
    assert "hunter2" not in result["error_message"]
    assert "raw_token" not in result["error_message"]
    assert "[REDACTED]" in result["error_message"]


@patch("subprocess.run")
def test_cli_run_pipeline_non_json_failure_uses_cli_output(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=1,
        stdout="not json",
        stderr="",
    )

    result = bridge.run_pipeline("config.yaml")

    assert isinstance(result, ExtendedDict)
    assert result["success"] is False
    assert result["error_message"] == "not json"


@patch("subprocess.run")
def test_cli_run_pipeline_non_json_failure_redacts_cli_output(
    mock_run: MagicMock,
    bridge: SecretsSyncBridge,
) -> None:
    mock_run.return_value = MagicMock(
        returncode=1,
        stdout="",
        stderr="failed password=hunter2 Authorization: Bearer raw_token",
    )

    result = bridge.run_pipeline("config.yaml")

    assert result["success"] is False
    assert "hunter2" not in result["error_message"]
    assert "raw_token" not in result["error_message"]
    assert "[REDACTED]" in result["error_message"]


@patch("subprocess.run")
def test_cli_run_pipeline_only_emits_supported_cli_flags(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout=json.dumps({"success": True}),
        stderr="",
    )

    options = SyncOptions(
        targets=["prod", "staging"],
        continue_on_error=False,
        parallelism=12,
    )
    bridge.run_pipeline("config.yaml", options)

    args = mock_run.call_args[0][0]
    assert "--targets" in args
    assert args[args.index("--targets") + 1] == "prod,staging"
    assert "--parallelism" in args
    assert args[args.index("--parallelism") + 1] == "12"
    assert "--continue-on-error=false" in args


@patch("subprocess.run")
def test_cli_validate_config(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=0,
        stdout="Valid",
        stderr="",
    )

    validation = bridge.validate_config("config.yaml")
    assert isinstance(validation, ExtendedDict)
    assert validation["valid"] is True
    assert "valid" in validation["message"].lower()

    args = mock_run.call_args[0][0]
    assert "validate" in args


@patch("subprocess.run")
def test_cli_validate_config_redacts_cli_output(mock_run: MagicMock, bridge: SecretsSyncBridge) -> None:
    mock_run.return_value = MagicMock(
        returncode=1,
        stdout="",
        stderr="invalid password=hunter2 Authorization: Bearer raw_token",
    )

    validation = bridge.validate_config("config.yaml")

    assert validation["valid"] is False
    assert "hunter2" not in validation["message"]
    assert "raw_token" not in validation["message"]
    assert "[REDACTED]" in validation["message"]
