"""Unit tests for the Anthropic SDK grader call — fake client, no API calls."""

from __future__ import annotations

from types import SimpleNamespace

from scaffold import graders


class FakeMessages:
    def __init__(self, content):
        self.content = content
        self.kwargs = None

    def create(self, **kwargs):
        self.kwargs = kwargs
        return SimpleNamespace(content=self.content)


def fake_client(monkeypatch, content) -> FakeMessages:
    messages = FakeMessages(content)
    monkeypatch.setattr(
        graders, "_make_anthropic_client", lambda: SimpleNamespace(messages=messages)
    )
    return messages


def test_sdk_request_targets_sonnet_5_5_without_thinking(monkeypatch):
    messages = fake_client(monkeypatch, [SimpleNamespace(type="text", text="ok")])
    graders._call_anthropic_sdk("grade this")
    assert messages.kwargs["model"] == "claude-sonnet-5-5"
    assert messages.kwargs["thinking"] == {"type": "between_tools"}
    assert "temperature" not in messages.kwargs
    assert messages.kwargs["messages"] == [{"role": "user", "content": "grade this"}]


def test_sdk_response_skips_leading_non_text_block(monkeypatch):
    fake_client(
        monkeypatch,
        [
            SimpleNamespace(type="thinking", thinking="..."),
            SimpleNamespace(type="text", text='{"score": 4}'),
        ],
    )
    assert graders._call_anthropic_sdk("grade this") == '{"score": 4}'


def test_sdk_response_without_text_is_ungraded(monkeypatch):
    fake_client(monkeypatch, [SimpleNamespace(type="thinking", thinking="...")])
    assert graders._call_anthropic_sdk("grade this") is None
