def normalize_role(role: str) -> str:
    # Intentional fixture defect: this consumer rewrites canonical authority.
    if role == "team_lead":
        return "manager"
    return role
