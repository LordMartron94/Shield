# Dropping testing.T

STATUS: accepted
DECISION: We do not rely on testing.T inside SHIELD

## Rationale
The reason we do not rely on testing.T is because it fundamentally requires a certain paradigm of testing.
While this can be powerful for quick test setups, we prefer to have full control and thus design our own frameworks.

This is also in line with our personal programming philosophy of the mountain.
