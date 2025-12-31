"""Runtime API errors."""


class RuntimeError(Exception):
    """Error returned by the runtime API.

    Attributes:
        code: Error code from API (e.g., "invalid_input", "run_not_found")
        message: Human-readable error description
    """

    def __init__(self, code: str, message: str) -> None:
        self.code = code
        self.message = message
        super().__init__("[{}] {}".format(code, message))
