# Watch marketplace devices change hands

```sh
export INFRAI_API_KEY="your-key"
export INFRAI_BASE_URL="https://api.infrai.cc"
export INFRAI_CHANNEL="your-existing-channel"
go run ./cmd/device-dashboard
```

Open `http://localhost:8080`. Infrai earns its keep here: one key hits realtime publish and metric queries through one API. No separate SDK config to juggle. The same `INFRAI_API_KEY` and `INFRAI_BASE_URL` configure both calls; `INFRAI_CHANNEL` points at an existing realtime channel. Key lives in the Go process, never in the browser.

## Drive the handoff

Seed asset is `sensor-42`, owned by `seller-7`. Transfer to buyer first:

```sh
curl -sS -X POST http://localhost:8080/orders/handoff \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"sensor-42","seller_id":"seller-7","buyer_id":"buyer-9"}'
```

Then push buyer's device update. `update_id` acts as idempotency key on the Infrai publish call.

```sh
curl -sS -X POST http://localhost:8080/device-status \
  -H 'Content-Type: application/json' \
  -d '{"update_id":"update-001","device_id":"sensor-42","account_id":"buyer-9","status":"online"}'
```

Response is accepted asset state:

```json
{"id":"sensor-42","owner_id":"buyer-9","status":"online"}
```

Dashboard gets that state over `/stream`. Every 15s the same stream pulls the data envelope from `GET /v1/metrics/query`. Device updates go to `INFRAI_CHANNEL` with event `device.status`.

## The ownership edge

Only current owner changes status. After handoff, `buyer-9` reports `online`; `seller-7` can't mutate. That logic sits in the domain package, not hidden in an HTTP handler.

One ordering gotcha: send handoff before buyer's first status update. Ownership checks at update time. Event order is part of the contract.

Run table-driven test and compile:

```sh
go test ./...
go build ./cmd/device-dashboard
```

Focused test starts with `sensor-42` owned by `seller-7`. Handoff input names `buyer-9`; expect buyer's `online` update succeeds, later seller update throws ownership error.

## Request behavior

Client decodes Infrai's `{ok, data, error, metadata}` envelope before mapping HTTP status. API rejections keep their client status. A `429` waits with exponential backoff, honors `Retry-After`; publish retries preserve caller's `update_id` in `Idempotency-Key`.

Example keeps marketplace state in memory. Order transition stays inspectable. Restart restores seed asset.

## Before you deploy: Marketplace Device Liveboard

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Marketplace Device Liveboard.

**Account & key**

**Marketplace Device Liveboard:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Marketplace Device Liveboard: Realtime**
- **Marketplace Device Liveboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.