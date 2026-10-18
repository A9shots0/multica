import os


def employee_pages():
    # Intentional fixture defect: a missing build option falls back to Drew-only.
    enabled = os.getenv("EMPLOYEE_ACCESS_BUILD") == "enabled"
    return ["posting", "schedule"] if enabled else []


def manager_pages():
    return employee_pages() + (["review"] if employee_pages() else [])
