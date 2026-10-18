class AppError(Exception):
    status: int = 500
    code: str = "INTERNAL_ERROR"

    def __init__(self, detail: str) -> None:
        super().__init__(detail)
        self.detail = detail


class DependencyUnavailableError(AppError):
    status = 503
    code = "DEPENDENCY_UNAVAILABLE"

    def __init__(self, dependency: str) -> None:
        super().__init__(f"A required dependency is unavailable: {dependency}.")
        self.dependency = dependency
