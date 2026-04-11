"""Sample Python module for testing tree-sitter chunking."""

import os
from typing import List, Optional


class UserService:
    """Manages user operations."""

    def __init__(self, db):
        self.db = db

    def get_user(self, user_id: int) -> Optional[dict]:
        """Get a user by ID."""
        return self.db.find(user_id)

    async def update_user(self, user_id: int, data: dict) -> bool:
        """Update user data."""
        return await self.db.update(user_id, data)


@staticmethod
def standalone_function(x: int, y: int) -> int:
    """A standalone function."""
    return x + y


def nested_example():
    """Function with nested function."""
    def inner():
        pass
    return inner()
