"""Runtime API client."""

import json
import urllib.request
import urllib.error
from typing import Any, Dict, Optional

from .errors import RuntimeError


class RuntimeClient:
    """Synchronous client for the runtime sidecar API.

    This client is synchronous and blocking by design.
    It treats StartRunRequest as an opaque dict - no schema validation.
    All requests include X-Runtime-Version: v1 header.

    Example:
        client = RuntimeClient("http://localhost:8080")
        response = client.start_run({
            "policy": {...},
            "tasks": [...]
        })
        print(response["id"], response["state"])
    """

    _VERSION_HEADER = "X-Runtime-Version"
    _VERSION = "v1"

    def __init__(self, base_url: str) -> None:
        """Initialize client with runtime sidecar URL.

        Args:
            base_url: Runtime sidecar URL (e.g., "http://localhost:8080")
        """
        self._base_url = base_url.rstrip("/")

    def start_run(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """Start a new run.

        Sends the request dict as-is to POST /api/v1/runs.
        No validation or modification of the request structure.

        Args:
            request: StartRunRequest as opaque dict

        Returns:
            Run response with id, state, tasks, etc.

        Raises:
            RuntimeError: On HTTP >= 400
        """
        return self._post("/api/v1/runs", request)

    def get_status(self, run_id: str) -> Dict[str, Any]:
        """Get run status.

        Args:
            run_id: Run identifier

        Returns:
            Run response with current state

        Raises:
            RuntimeError: On HTTP >= 400 (e.g., run_not_found)
        """
        return self._get("/api/v1/runs/{}".format(run_id))

    def abort_run(self, run_id: str) -> None:
        """Abort a running run.

        Args:
            run_id: Run identifier

        Raises:
            RuntimeError: On HTTP >= 400 (e.g., run_completed)
        """
        self._post("/api/v1/runs/{}/abort".format(run_id), None)

    def _get(self, path: str) -> Dict[str, Any]:
        """Execute GET request."""
        url = self._base_url + path
        req = urllib.request.Request(url, method="GET")
        req.add_header(self._VERSION_HEADER, self._VERSION)
        return self._execute(req)

    def _post(self, path: str, body: Optional[Dict[str, Any]]) -> Dict[str, Any]:
        """Execute POST request."""
        url = self._base_url + path
        data = json.dumps(body).encode("utf-8") if body else None
        req = urllib.request.Request(url, data=data, method="POST")
        req.add_header(self._VERSION_HEADER, self._VERSION)
        if data:
            req.add_header("Content-Type", "application/json")
        return self._execute(req)

    def _execute(self, req: urllib.request.Request) -> Dict[str, Any]:
        """Execute request and handle errors."""
        try:
            with urllib.request.urlopen(req) as resp:
                body = resp.read().decode("utf-8")
                return json.loads(body) if body else {}
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8")
            try:
                err = json.loads(body)
                raise RuntimeError(err.get("code", "unknown"), err.get("message", body))
            except json.JSONDecodeError:
                raise RuntimeError("http_error", "HTTP {}: {}".format(e.code, body))
