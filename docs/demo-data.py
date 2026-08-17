#!/usr/bin/env python3
"""Stage synthetic session logs for the demo recording (see demo.tape).

Writes fake-but-realistic Claude Code and Codex JSONL logs to
/tmp/mtok-demo/{claude,codex} so the demo GIF never shows real project
names or spend. Deterministic except for the anchor date (today), so the
dashboard's TODAY tile is always live.

    python3 docs/demo-data.py
"""

import json
import random
import shutil
import uuid
from datetime import datetime, timedelta
from pathlib import Path

ROOT = Path("/tmp/mtok-demo")
DAYS = 100  # ~4 calendar months of history

random.seed(42)

# Weighted fake projects; cwds live under the real $HOME so the TUI
# shortens them to ~/code/... like real entries.
HOME = str(Path.home())
CLAUDE_PROJECTS = [
    (f"{HOME}/code/api-server", 30),
    (f"{HOME}/code/webapp", 25),
    (f"{HOME}/code/ml-eval", 18),
    (f"{HOME}/code/infra", 12),
    (f"{HOME}/code/cli-tools", 9),
    (f"{HOME}/dotfiles", 6),
]
CODEX_PROJECTS = [
    (f"{HOME}/code/data-pipeline", 60),
    (f"{HOME}/code/webapp", 40),
]


def pick(weighted):
    total = sum(w for _, w in weighted)
    r = random.uniform(0, total)
    for v, w in weighted:
        r -= w
        if r <= 0:
            return v
    return weighted[-1][0]


def claude_model(day_age):
    # Fable 5 takes over ~7 weeks back; before that an Opus/Sonnet mix.
    if day_age < 50:
        return pick([("claude-fable-5", 62), ("claude-sonnet-5", 22),
                     ("claude-opus-4-8", 9), ("claude-haiku-4-5", 7)])
    return pick([("claude-opus-4-8", 48), ("claude-sonnet-4-6", 38),
                 ("claude-haiku-4-5", 14)])


def iso(dt):
    return dt.astimezone().isoformat(timespec="seconds")


def day_intensity(day_age, weekday):
    base = 1.0 + 0.35 * random.uniform(-1, 1)
    if weekday >= 5:
        base *= 0.35
    if day_age == 6:
        base *= 2.3  # the big-refactor spike in the 30-day chart
    if day_age == 15:
        base *= 1.7
    return base


seq = 0


def claude_session(f, start, project, model, turns):
    global seq
    sid = str(uuid.uuid4())
    t = start
    context = random.randint(8_000, 15_000)
    rows = []
    for _ in range(turns):
        seq += 1
        write = (random.randint(8_000, 22_000) if random.random() < 0.06
                 else random.randint(300, 3_000))
        out_r = random.random()
        if out_r < 0.80:
            output = random.randint(150, 1_200)
        elif out_r < 0.95:
            output = random.randint(1_500, 4_000)
        else:
            output = random.randint(5_000, 9_000)
        w5m = int(write * random.uniform(0.6, 0.95))
        usage = {
            "input_tokens": random.randint(3, 12),
            "cache_creation_input_tokens": write,
            "cache_read_input_tokens": context,
            "output_tokens": output,
            "cache_creation": {
                "ephemeral_5m_input_tokens": w5m,
                "ephemeral_1h_input_tokens": write - w5m,
            },
        }
        if model.startswith(("claude-fable", "claude-opus")) and random.random() < 0.6:
            usage["output_tokens_details"] = {
                "thinking_tokens": int(output * random.uniform(0.05, 0.4))
            }
        rows.append({
            "type": "assistant",
            "timestamp": iso(t),
            "requestId": f"req_{seq:08d}",
            "sessionId": sid,
            "cwd": project,
            "message": {"id": f"msg_{seq:08d}", "model": model, "usage": usage},
        })
        context += write
        if context > 160_000:  # compaction
            context = random.randint(20_000, 30_000)
        t += timedelta(seconds=random.randint(15, 90))
    for r in rows:
        f.write(json.dumps(r, separators=(",", ":")) + "\n")
    return sid, rows


def codex_session(path, start, project, model, events):
    t = start
    total = {"input_tokens": 0, "cached_input_tokens": 0,
             "cache_write_input_tokens": 0, "output_tokens": 0,
             "reasoning_output_tokens": 0}
    with path.open("w") as f:
        f.write(json.dumps({
            "timestamp": iso(t), "type": "session_meta",
            "payload": {"id": str(uuid.uuid4()), "cwd": project,
                        "model_provider": "openai"},
        }) + "\n")
        f.write(json.dumps({
            "timestamp": iso(t), "type": "turn_context",
            "payload": {"model": model},
        }) + "\n")
        for _ in range(events):
            cached = random.randint(20_000, 90_000)
            fresh = random.randint(300, 3_000)
            output = random.randint(200, 2_500)
            total["cached_input_tokens"] += cached
            total["input_tokens"] += cached + fresh
            total["output_tokens"] += output
            total["reasoning_output_tokens"] += int(output * random.uniform(0.3, 0.7))
            t += timedelta(seconds=random.randint(20, 120))
            f.write(json.dumps({
                "timestamp": iso(t), "type": "event_msg",
                "payload": {"type": "token_count",
                            "info": {"total_token_usage": dict(total)}},
            }) + "\n")


def flatten(cwd):
    return cwd.replace("/", "-")


def main():
    shutil.rmtree(ROOT, ignore_errors=True)
    today = datetime.now().replace(hour=0, minute=0, second=0, microsecond=0)

    resume_pool = []  # (dir, rows) to replay as resumed-session copies
    for day_age in range(DAYS, -1, -1):
        day = today - timedelta(days=day_age)
        intensity = day_intensity(day_age, day.weekday())

        n_sessions = max(0, round(random.randint(3, 6) * intensity))
        if day_age == 0:
            n_sessions = max(2, n_sessions // 2)  # today is still in progress
        for _ in range(n_sessions):
            project = pick(CLAUDE_PROJECTS)
            model = claude_model(day_age)
            turns = max(8, int(random.lognormvariate(4.4, 0.6) * intensity))
            start = day + timedelta(hours=random.randint(9, 21),
                                    minutes=random.randint(0, 59))
            d = ROOT / "claude" / "projects" / flatten(project)
            d.mkdir(parents=True, exist_ok=True)
            with (d / f"{uuid.uuid4()}.jsonl").open("w") as f:
                _, rows = claude_session(f, start, project, model, turns)
            if random.random() < 0.04:
                resume_pool.append((d, rows))

        for _ in range(random.randint(0, 2)):
            project = pick(CODEX_PROJECTS)
            model = "gpt-5.3-codex" if day_age >= 20 or random.random() < 0.5 else "gpt-5.6"
            start = day + timedelta(hours=random.randint(9, 21),
                                    minutes=random.randint(0, 59))
            d = ROOT / "codex" / "sessions" / f"{day:%Y/%m/%d}"
            d.mkdir(parents=True, exist_ok=True)
            name = f"rollout-{start:%Y-%m-%dT%H-%M-%S}-{uuid.uuid4()}.jsonl"
            codex_session(d / name, start, project, model,
                          random.randint(20, 80))

    # Resumed sessions copy history into a new file; the scanner should
    # dedup these — the footer's "deduped" stat comes from here.
    for d, rows in resume_pool:
        with (d / f"{uuid.uuid4()}.jsonl").open("w") as f:
            for r in rows[: len(rows) // 2]:
                f.write(json.dumps(r, separators=(",", ":")) + "\n")

    files = sum(1 for _ in ROOT.rglob("*.jsonl"))
    print(f"staged {files} files under {ROOT}")


if __name__ == "__main__":
    main()
