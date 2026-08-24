"""Host-side payload fixture; runs without CircuitPython hardware modules."""

from payload import moisture_payload


assert moisture_payload(32768, 50) == '{"raw":32768,"percentage":50}'
for values in ((-1, 50), (1, -1), (1, 101), (1.5, 50), (1, 50.5)):
    try:
        moisture_payload(*values)
    except ValueError:
        pass
    else:
        raise AssertionError("invalid values accepted: %r" % (values,))
