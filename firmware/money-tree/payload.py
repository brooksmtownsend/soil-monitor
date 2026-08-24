"""Wire-format helpers shared by firmware and host-side fixtures."""


def moisture_payload(raw, percentage):
    """Return the compact JSON moisture payload expected by the monitor."""
    if not isinstance(raw, int) or raw < 0:
        raise ValueError("raw must be a non-negative integer")
    if not isinstance(percentage, int) or not 0 <= percentage <= 100:
        raise ValueError("percentage must be an integer from 0 through 100")
    return '{"raw":%d,"percentage":%d}' % (raw, percentage)
