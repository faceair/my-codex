#!/usr/bin/env python3
import json
import re
import sys
import time
from pathlib import Path
from typing import List, Optional, Set, Tuple


MAX_PLAN_AGE_SECONDS = 24 * 60 * 60
OPEN_ITEM_RE = re.compile(r"^\s*[-*]\s*\[( |>|!)\]\s+(.+)$", re.MULTILINE)
FINAL_STATUS_RE = re.compile(r"final status\s*[:：]\s*(\S+)", re.IGNORECASE)
PLAN_REF_RE = re.compile(
    r"(?P<path>(?:[A-Za-z]:)?[^\s`'\"]*?\.codex/plans/(?P<slug1>[a-z0-9][a-z0-9-]*)\.md)"
    r"|(?P<rel>plans/(?P<slug2>[a-z0-9][a-z0-9-]*)\.md)"
    r"|#\s*Plan:\s*(?P<slug3>[a-z0-9][a-z0-9-]*)",
    re.IGNORECASE,
)


def load_payload() -> dict:
    raw = sys.stdin.read().strip()
    return json.loads(raw) if raw else {}


def plan_dirs_for(cwd: Path) -> List[Path]:
    candidates: List[Path] = []
    if cwd.name == ".codex":
        candidates.append(cwd / "plans")

    candidates.append(cwd / ".codex" / "plans")
    candidates.append(cwd / "plans")

    unique: List[Path] = []
    seen: Set[Path] = set()
    for path in candidates:
        resolved = path.resolve()
        if resolved not in seen and resolved.is_dir():
            seen.add(resolved)
            unique.append(resolved)
    return unique


def final_status(text: str) -> Optional[str]:
    match = FINAL_STATUS_RE.search(text)
    return match.group(1).strip().lower() if match else None


def transcript_texts(transcript_path: Path) -> List[Tuple[str, str]]:
    results: List[Tuple[str, str]] = []
    try:
        with transcript_path.open(encoding="utf-8") as f:
            for line in f:
                obj = json.loads(line)
                payload = obj.get("payload") or {}
                if (
                    obj.get("type") == "event_msg"
                    and payload.get("type") == "agent_message"
                    and isinstance(payload.get("message"), str)
                ):
                    results.append(("agent_message", payload["message"]))
                    continue

                if (
                    obj.get("type") == "response_item"
                    and payload.get("role") == "assistant"
                    and isinstance(payload.get("content"), list)
                ):
                    parts: List[str] = []
                    for item in payload["content"]:
                        if isinstance(item, dict):
                            text = item.get("text") or item.get("content")
                            if isinstance(text, str):
                                parts.append(text)
                    if parts:
                        results.append(("assistant", "\n".join(parts)))
    except Exception:
        return []
    return results


def is_subagent_session(transcript_path: Path) -> bool:
    try:
        with transcript_path.open(encoding="utf-8") as f:
            first_line = f.readline()
        if not first_line:
            return False
        obj = json.loads(first_line)
        payload = obj.get("payload") or {}
        source = payload.get("source")
        return isinstance(source, dict) and "subagent" in source
    except Exception:
        return False


def latest_plan_ref_from_transcript(
    transcript_path: Path,
) -> Optional[Tuple[Optional[Path], str]]:
    latest_path: Optional[Path] = None
    latest_slug: Optional[str] = None

    for _, text in transcript_texts(transcript_path):
        for match in PLAN_REF_RE.finditer(text):
            path_str = match.group("path")
            slug = (
                match.group("slug1")
                or match.group("slug2")
                or match.group("slug3")
            )
            if not slug:
                continue
            latest_slug = slug
            latest_path = Path(path_str).expanduser() if path_str else None

    if not latest_slug:
        return None
    return latest_path, latest_slug


def plan_summary(plan_path: Path) -> Optional[Tuple[int, List[str]]]:
    try:
        stat = plan_path.stat()
        if (time.time() - stat.st_mtime) > MAX_PLAN_AGE_SECONDS:
            return None
        text = plan_path.read_text(encoding="utf-8")
    except Exception:
        return None

    status = final_status(text)
    if status in {"done", "cancelled", "completed"}:
        return None

    open_items = [m.group(2).strip() for m in OPEN_ITEM_RE.finditer(text)]
    if not open_items:
        return None

    return len(open_items), open_items[:3]


def resolve_plan_path(
    cwd: Path,
    transcript_plan_path: Optional[Path],
    transcript_slug: Optional[str],
) -> Optional[Path]:
    allowed_plan_dirs = plan_dirs_for(cwd)
    allowed_plan_dir_set: Set[Path] = {path.resolve() for path in allowed_plan_dirs}

    if transcript_plan_path:
        candidates = [transcript_plan_path]
        if not transcript_plan_path.is_absolute():
            candidates.append((cwd / transcript_plan_path).resolve())
        for candidate in candidates:
            try:
                resolved = candidate.resolve()
                parent = resolved.parent.resolve()
            except Exception:
                continue
            if resolved.is_file() and parent in allowed_plan_dir_set:
                return resolved

    if transcript_slug:
        for plan_dir in allowed_plan_dirs:
            candidate = (plan_dir / f"{transcript_slug}.md").resolve()
            if candidate.is_file():
                return candidate

    return None


def main() -> int:
    payload = load_payload()

    if payload.get("hook_event_name") != "Stop":
        return 0

    cwd_raw = payload.get("cwd")
    if not cwd_raw:
        return 0
    cwd = Path(cwd_raw).resolve()

    transcript_path_raw = payload.get("transcript_path")
    if not transcript_path_raw:
        return 0
    transcript_path = Path(transcript_path_raw)
    if is_subagent_session(transcript_path):
        return 0

    transcript_info = latest_plan_ref_from_transcript(transcript_path)
    if not transcript_info:
        return 0

    transcript_plan_path, transcript_slug = transcript_info
    active_plan_path = resolve_plan_path(cwd, transcript_plan_path, transcript_slug)
    if not active_plan_path:
        return 0

    summary = plan_summary(active_plan_path)
    if not summary:
        return 0

    open_count, examples = summary
    preview = "；".join(examples)
    reason = (
        f"继续当前任务。检测到当前活跃计划 {active_plan_path.name} 仍有 {open_count} 个未完成项。"
        f"优先完成当前 active milestone 或明确记录 blocker 后再停止。"
        f"未完成项示例：{preview}"
    )
    print(json.dumps({"decision": "block", "reason": reason}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
