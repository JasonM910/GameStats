"""Smoke test de las imágenes reales; reutiliza un lote generado por Go."""
import json
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/")
with open(sys.argv[2], encoding="utf-8") as source:
    document = json.load(source)
events = [event for match in document["matches"] for event in match["events"]]

for attempt in range(60):
    try:
        with urllib.request.urlopen(base + "/health", timeout=5) as response:
            assert response.status == 200
        break
    except (urllib.error.URLError, TimeoutError):
        if attempt == 59:
            raise
        time.sleep(1)

for _ in range(2):
    for offset in range(0, len(events), 100):
        batch = events[offset:offset + 100]
        request = urllib.request.Request(base + "/api/events", data=json.dumps(batch).encode(), headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(request, timeout=20) as response:
            assert json.load(response)["eventIds"] == [event["eventId"] for event in batch]

with urllib.request.urlopen(base + "/api/stats", timeout=20) as response:
    result = json.load(response)
assert result["schemaVersion"] == 1
assert len(result["matches"]) == len(document["matches"])
assert sum(match["eventCount"] for match in result["matches"]) == len(events)
assert result["metrics"] and result["players"]
assert all(match["result"] != "UNFINISHED" for match in result["matches"])
print("API: eventos, reintentos y métricas Scala correctos")
