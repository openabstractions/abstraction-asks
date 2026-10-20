"""Public Python notification client through the Go test's supplied frame transport."""
import base64
import dataclasses
import sys
import time

from abstraction.asks.api import (NotificationAction, NotificationActivation, NotificationApplicationClient,
                                  NotificationDelivery, NotificationRequest, RequestRef)


class Transport:
    def __init__(self):
        self.frames = []

    def exchange_frame(self, frame):
        self.frames.append(frame)
        print(base64.b64encode(frame).decode("ascii"), flush=True)
        return base64.b64decode(sys.stdin.readline())


transport = Transport()
client = NotificationApplicationClient(transport)
now = int(time.time() * 1000)
ref = RequestRef(key="python-finished", replay_until_ms=now + 3600000)
request = NotificationRequest(request_ref=ref, title="Picture ready", body="cat.png is ready.",
                              actions=[NotificationAction(id="open", title="Open", activation=NotificationActivation.APP)],
                              expires_ms=now + 600000, delivery=NotificationDelivery.NOW)
first = client.post(request)
assert first.outcome == "posted" and first.id and first.result is None
assert client.post(request) == first and transport.frames[0] == transport.frames[1]
assert client.post(dataclasses.replace(request, title="Changed")).outcome == "conflict"
# The Go side records the person's action between these frames.
acted = client.observe(ref, 0)
assert acted.outcome == "acted" and acted.result.action == "open" and acted.result.decided_by
assert client.acknowledge(ref, acted.result).outcome == "acknowledged"
assert client.observe(ref, 0).outcome == "gone"
other = RequestRef(key="python-withdrawn", replay_until_ms=ref.replay_until_ms)
assert client.post(dataclasses.replace(request, request_ref=other)).outcome == "posted"
withdrawn = client.withdraw(other)
assert withdrawn.outcome == "withdrawn" and not withdrawn.result.action
assert client.post(dataclasses.replace(request, request_ref=RequestRef(key="invalid", replay_until_ms=ref.replay_until_ms), title="")).outcome == "invalid"
