# GoZon microservices (Go + RabbitMQ + Postgres)

Домашняя работа №4: асинхронное межсервисное взаимодействие. Вариант на максимальный балл с фронтендом.

Автор: Воронин Глеб Дмитриевич, группа БПИ243.

## Архитектура
- `orders-service` — REST заказов, transactional outbox (DB) → очередь `order.payments`, consumer статусов оплаты.
- `payments-service` — счета/баланс, transactional inbox + outbox, consumer `order.payments`, publisher `payment.status`, идемпотентное списание.
- `gateway` — reverse proxy (`/orders`, `/payments`, остальное → фронт).
- `frontend` — лёгкий SPA на чистом JS (fetch к gateway).
- Инфраструктура: `rabbitmq`, `orders-db` (Postgres), `payments-db` (Postgres).

Messaging:
- Очереди: `order.payments` (tasks), `payment.status` (results).
- At-least-once доставка (durable очереди, manual ack).
- Exactly-once семантика списаний: inbox dedup по `message_id`, уникальный `order_id` в платежах, `FOR UPDATE` по балансу, outbox для отправки результата.

Ключевые паттерны:
- Transactional Outbox: оба сервиса (паблишинг из таблицы `outbox` фоновой джобой).
- Transactional Inbox: payments (таблица `inbox` + upsert).
- Идемпотентные обработчики: повторные сообщения не меняют баланс и статус заказа.

## Запуск
```bash
# из корня репо
docker compose up --build
# gateway:     http://localhost:8080
# frontend UI: http://localhost:8080/
# RabbitMQ UI: http://localhost:15672 (guest/guest)
```

## Быстрый сценарий (через HTTP или UI)
1) Создать счёт  
`POST /payments/accounts { "user_id": "user-1" }`

2) Пополнить счёт  
`POST /payments/accounts/deposit { "user_id": "user-1", "amount": 2000 }`

3) Создать заказ (запускает оплату асинхронно)  
`POST /orders { "user_id": "user-1", "amount": 1500, "description": "Gift" }`

4) Проверить заказы  
`GET /orders?user_id=user-1`

5) Проверить баланс  
`GET /payments/accounts/user-1/balance`

Фронт: открыть `http://localhost:8080/`, заполнить `user_id`, пополнить, создать заказ, смотреть лог и список заказов.

## Документация и примеры
- OpenAPI: `docs/openapi.yaml`
- Примеры запросов: `docs/requests.http`
- Очереди/топики: `order.payments`, `payment.status`

## Стек и версии
- Go 1.22, RabbitMQ 3-management, Postgres 15, Docker Compose.
- Библиотеки: chi, pgx, amqp091-go, uuid.

## Структура сервисов (слои)
- `internal/config` — конфиг из env.
- `internal/db` — миграции схемы.
- `internal/*/repository` — работа с БД.
- `internal/*/service` — бизнес-логика.
- `internal/http` — роутеры/handlers.
- `internal/mq` — consumers/publishers outbox.

## Тест/проверка работоспособности
- `docker compose up --build`
- Через UI или `docs/requests.http` прогнать сценарий: счёт → пополнение → заказ → статус FINISHED → баланс уменьшился.
- Повторная публикация того же сообщения не должна менять баланс/статус (idempotency).

## Прочее
- `*.exe` артефакты сборки под Windows — удалять, добавить в `.gitignore`, в образ не попадают.***

