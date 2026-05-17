# /dashboard

Next.js 14 dashboard. The face of Citadel — what judges see when we demo.

## Stack

- Next.js 14 (App Router, TypeScript)
- Tailwind CSS
- `lucide-react` for icons
- Optional `react-flow` for the process tree

## Pages

- `/runs` — recent workflow runs with event/detection counts
- `/runs/[id]` — run detail: tabs for Network · Processes · Files · Timeline, plus a sticky Detections sidebar
- `/policies` — policy list + editor (audit/block mode, allowlists, severity→action map)
- `/detections` — global feed (optional)

## Config

`NEXT_PUBLIC_BACKEND_URL` — backend base URL (default `http://localhost:8080`).

## Dev

```sh
npm install
npm run dev   # http://localhost:3000
```
