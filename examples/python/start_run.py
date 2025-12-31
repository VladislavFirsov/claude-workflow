#!/usr/bin/env python3
"""Example: Start a run using the Python SDK.

Usage:
    export PYTHONPATH="${PYTHONPATH}:$(pwd)/sdk/python"
    python examples/python/start_run.py
"""

from claude_workflow import RuntimeClient, RuntimeError


def main() -> None:
    """Run example."""
    # Create client (assumes sidecar running on localhost:8080)
    client = RuntimeClient("http://localhost:8080")

    # Build request (opaque dict - SDK doesn't validate structure)
    request = {
        "id": "python-sdk-example",
        "policy": {
            "timeout_ms": 300000,
            "max_parallelism": 2,
            "budget_limit": {"amount": 5.0, "currency": "USD"}
        },
        "tasks": [
            {
                "id": "analysis",
                "prompt": "Analyze the requirements",
                "model": "claude-sonnet-4-20250514",
                "metadata": {"role": "spec-analyst"}
            },
            {
                "id": "architecture",
                "prompt": "Design the architecture",
                "model": "claude-sonnet-4-20250514",
                "deps": ["analysis"],
                "metadata": {"role": "spec-architect"}
            }
        ]
    }

    try:
        # Start run
        response = client.start_run(request)
        print("Started run: id={} state={}".format(response["id"], response["state"]))

        # Get status
        status = client.get_status(response["id"])
        print("Status: state={}".format(status["state"]))
        for task_id, task in status.get("tasks", {}).items():
            print("  {}: {}".format(task_id, task["state"]))

    except RuntimeError as e:
        print("Error: [{}] {}".format(e.code, e.message))


if __name__ == "__main__":
    main()
