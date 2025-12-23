# GoZon microservices (Go + RabbitMQ + Postgres)

Домашнее задание №4: Orders + Payments с transactional outbox/inbox и минимальным фронтендом.

## Сервисы
- `orders-service` – CRUD заказов, transactional outbox -> очередь `order.payments`, consumer статусов оплаты.
- `payments-service` – счета пользователей, пополнение, consumer `order.payments`, transactional inbox/outbox, идемпотентное списание (unique `order_id`, блокировки баланса), publisher статусов в `payment.status`.
- `gateway` – простой reverse proxy (`/orders`, `/payments`, все остальное -> фронт).
- `frontend` – маленький SPA на чистом JS (fetch к gateway).
- Инфраструктура: `rabbitmq`, `orders-db` (Postgres), `payments-db` (Postgres).

## Запуск
```bash
docker compose up --build
# gateway доступен на http://localhost:8080
# rabbitmq UI: http://localhost:15672 (guest/guest)
```

## Быстрый сценарий
1. Создать счет: `POST /payments/accounts { "user_id": "user-1" }`
2. Пополнить: `POST /payments/accounts/deposit { "user_id": "user-1", "amount": 2000 }`
3. Создать заказ: `POST /orders { "user_id": "user-1", "amount": 1500, "description": "Gift" }`
4. Проверить список заказов: `GET /orders?user_id=user-1`
5. Посмотреть баланс: `GET /payments/accounts/user-1/balance`

Коллекции/доки:
- OpenAPI: `docs/openapi.yaml`
- Примеры запросов для REST-клиентов: `docs/requests.http`
- UI: `http://localhost:8080/` (через gateway)

## Технические детали
- Transactional Outbox в обоих сервисах (orders -> order.payments, payments -> payment.status).
- Transactional Inbox в payments (таблица `inbox` + уникальный `message_id`).
- Exactly-once списание за счет:
  - уникального `order_id` в таблице `payments`,
  - `SELECT ... FOR UPDATE` при работе с балансом,
  - dedup по `inbox.message_id`.
- Доставка сообщений at-least-once (durable queues + manual ack).
- Идемпотентность обработчиков: повторные сообщения не меняют баланс и статус заказа.

## Стек
Go 1.22, RabbitMQ, Postgres 15, Docker Compose, минимальный фронтенд на чистом JS/HTML.

