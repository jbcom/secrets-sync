"""Python bridge for SecretSync CLI and native runtimes."""

from __future__ import annotations


try:
    from importlib.metadata import version
except ImportError:  # pragma: no cover
    version = None  # type: ignore[assignment]

from secrets_sync.bridge import SecretsSyncBridge
from secrets_sync.models import (
    ConfigInfo,
    InfoLoggerLike,
    LifecycleLoggerLike,
    LoggerLike,
    OutputFormat,
    RuntimeBackend,
    SyncOperation,
    SyncOptions,
    SyncResult,
)


if version is None:  # pragma: no cover
    __version__ = "0.0.0"
else:
    try:
        __version__ = version("secrets-sync-bridge")
    except Exception:  # pragma: no cover
        __version__ = "0.0.0"


__all__ = [
    "ConfigInfo",
    "InfoLoggerLike",
    "LifecycleLoggerLike",
    "LoggerLike",
    "OutputFormat",
    "RuntimeBackend",
    "SecretsSyncBridge",
    "SyncOperation",
    "SyncOptions",
    "SyncResult",
    "__version__",
]
