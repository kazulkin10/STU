import logging
import os
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import httpx

logger = logging.getLogger("moderation-agent")
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO").upper())

app = FastAPI(title="Stu Moderation Agent")


class ReportPacket(BaseModel):
    report_id: str
    reporter_user_id: str
    reported_user_id: str
    reason: str
    message_text: str | None = None


class Decision(BaseModel):
    report_id: str
    verdict: str
    category: str | None = None
    confidence: float
    notes: str | None = None


@app.get("/healthz")
async def healthz():
    return {"status": "ok", "service": "moderation-agent"}


@app.post("/analyze", response_model=Decision)
async def analyze(packet: ReportPacket):
    if not packet.reason:
        raise HTTPException(status_code=400, detail="empty reason")
    verdict = await classify(packet)
    return verdict


async def classify(packet: ReportPacket) -> Decision:
    url = os.getenv(
        "TIMEWEB_AGENT_URL",
        "https://agent.timeweb.cloud/api/v1/cloud-ai/agents/6087a6cd-b070-4bcc-8b71-9aaed01c2168/v1",
    )
    api_key = os.getenv("TIMEWEB_AGENT_API_KEY", "")
    access_id = os.getenv("TIMEWEB_AGENT_ACCESS_ID", "")
    if not api_key:
        return Decision(
            report_id=packet.report_id,
            verdict="needs_review",
            category="other",
            confidence=0.1,
            notes="AI key missing",
        )
    system_prompt = (
        "Ты модератор. Верни JSON строго вида "
        '{"verdict":"allow|needs_review|ban_suspected","category":"spam|fraud|extremism|csam|hate|other","confidence":0..1,"notes":"кратко"}'
        ". Никакого другого текста."
    )
    user_content = (
        f"Репорт {packet.report_id}. Репортер {packet.reporter_user_id} на {packet.reported_user_id}. "
        f"Причина: {packet.reason}. "
        f"Текст: {packet.message_text or '(нет текста)'}"
    )
    payload = {
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_content},
        ],
        "max_tokens": 200,
    }
    headers = {"Authorization": f"Bearer {api_key}"}
    if access_id:
        headers["X-Access-Id"] = access_id
    async with httpx.AsyncClient(timeout=12.0) as client:
        try:
            resp = await client.post(url, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
        except Exception as exc:  # noqa: BLE001
            logger.warning("timeweb request failed: %s", exc)
            return Decision(
                report_id=packet.report_id,
                verdict="needs_review",
                category="other",
                confidence=0.2,
                notes="fallback",
            )
    try:
        content = data["choices"][0]["message"]["content"]
    except Exception:  # noqa: BLE001
        return Decision(
            report_id=packet.report_id,
            verdict="needs_review",
            category="other",
            confidence=0.2,
            notes="bad format",
        )
    import json

    try:
        parsed = json.loads(content)
    except Exception:
        try:
            parsed = json.loads(content.replace("```json", "").replace("```", ""))
        except Exception:
            parsed = {}
    return Decision(
        report_id=packet.report_id,
        verdict=parsed.get("verdict", "needs_review"),
        category=parsed.get("category", "other"),
        confidence=float(parsed.get("confidence", 0.5) or 0),
        notes=parsed.get("notes", "нет пояснения"),
    )


def main():
    import uvicorn

    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=int(os.getenv("PORT", "8085")),
        reload=False,
    )


if __name__ == "__main__":
    main()
