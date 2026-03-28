# Runtime Configuration - Filtering

STATUS: accepted
DECISION: For runtime filtering of units and atoms, only explicit blacklisting, no whitelisting.

## Rationale
The reason we choose for explicit blacklisting instead of whitelisting, despite seemingly worse ergonomics, is the fact that whitelisting increases the chance for unintentional skipping of tests significantly. By allowing only blacklisting, we force explicitness and thus reduce the chance for accidental skipping of behavior testing.

