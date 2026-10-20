"""Public Python choice client through the Go test's supplied frame transport."""
import base64
import dataclasses
import sys
import time

from abstraction.asks.api import ChoiceApplicationClient, ChoiceOption, ChoiceRequest, RequestRef


class Transport:
    def __init__(self):
        self.frames = []

    def exchange_frame(self, frame):
        self.frames.append(frame)
        print(base64.b64encode(frame).decode("ascii"), flush=True)
        return base64.b64decode(sys.stdin.readline())


transport = Transport()
client = ChoiceApplicationClient(transport)
now = int(time.time() * 1000)
ref = RequestRef(key="one-request-output", replay_until_ms=now + 3600000)
request = ChoiceRequest(request_ref=ref, title="Keep this output for this delivery?",
                        options=[ChoiceOption(id="keep", label="Keep")], pending_until_ms=now + 60000)
first = client.ask(request)
assert first.outcome == "pending" and first.answer is None
replay = client.ask(request)
assert replay == first and transport.frames[0] == transport.frames[1]
decision = client.answer(ref, "keep", True)
assert decision.outcome == "answered" and decision.record.request.request_ref == ref
assert decision.record.answer.option == "keep" and decision.record.answer.answered_by
assert client.answer(ref, "keep", True).outcome == "conflict"
assert client.answer(ref, "keep", False) == decision
answer = client.observe(ref, 0)
assert answer.outcome == "answered" and answer.answer == decision.record.answer
wrong = dataclasses.replace(answer.answer, answered_ms=answer.answer.answered_ms + 1)
assert client.acknowledge(ref, wrong).outcome == "conflict"
assert client.acknowledge(ref, answer.answer).outcome == "acknowledged"
assert client.acknowledge(ref, answer.answer).outcome == "acknowledged"
assert client.observe(ref, 0).outcome == "gone"
assert client.ask(dataclasses.replace(request, title="Changed intent")).outcome == "conflict"
new_ref = dataclasses.replace(ref, replay_until_ms=ref.replay_until_ms + 1)
assert client.ask(dataclasses.replace(request, request_ref=new_ref)).outcome == "pending"
closed = client.dismiss(new_ref)
assert closed.outcome == "dismissed" and not closed.record.answer.option
assert client.dismiss(new_ref) == closed
assert client.answer(new_ref, "keep", False).outcome == "conflict"
assert client.ask(dataclasses.replace(request, request_ref=RequestRef(key="invalid", replay_until_ms=ref.replay_until_ms), options=[])).outcome == "invalid"
