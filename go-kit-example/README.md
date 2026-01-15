# 🏗 Go-Kit Architecture Cheat Sheet

## 🔄 Поток данных (The Flow)
**Request** ➔ **Transport** ➔ **Endpoint** ➔ **Service** ➔ **Store**

---

## 🧩 Слои (Layers)

### 1. Transport (`http.go`) — "Курьер"
* **Задача:** Читает HTTP, парсит JSON, валидирует заголовки.
* **Логика:** Тупой переводчик "JSON ➔ Struct".
* **Если ошибка:** Отбивает `400 Bad Request` сразу, не пуская дальше.

### 2. Endpoint (`set.go`) — "Охрана / Фейс-контроль"
* **Задача:** Инфраструктурная защита и управление потоком.
* **Middleware (Технические):**
    * 🛑 **Rate Limiter:** Ограничивает RPS ("Не части!").
    * 🔌 **Circuit Breaker:** Защита от каскадных сбоев ("Сервис жив?").
    * ⏱ **Metrics:** Технический мониторинг ("Запрос длился 0.05с").

### 3. Service (`service.go` + `middleware.go`) — "Мозг"
* **Задача:** Чистая бизнес-логика.
* **Middleware (Бизнес):**
    * 📝 **Business Logging:** Видит суть ("User=55, Dist=100km").
    * 🧠 **Validation:** Проверка бизнес-правил.

### 4. Store (`store.go`) — "Архив"
* **Задача:** CRUD операции (Memory, DB). Исполнитель без логики.

---

## 🪆 Принцип Матрешки

Как запрос проникает вглубь приложения (визуализация "наслаивания"):

```text
[ TRANSPORT LAYER ] 
HTTP Handler (http.go)
 │
 ⬇ Распаковка JSON
 │
[ ENDPOINT LAYER - set.go ]
 ├── RateLimit Middleware       (Очередь: "Проходи по одному")
 │    └── CircuitBreaker MW     (Предохранитель: "Система в норме")
 │         └── Endpoint Metrics (Таймер: "Старт отсчета...")
 │              │
 │              ⬇ Передача в интерфейс Service
 │              │
[ SERVICE LAYER - middleware.go ]
 │              ├── Business Logging (Лог: "Пришли данные OBUID=99")
 │              │    └── Service Core (service.go: "Считаем математику...")
 │              │         │
 │              │         ⬇
[ STORAGE LAYER ]
 │              │         └── Store.Insert (Запись в память)
```

⚙️ Main.go — Сборочный цех (Wiring)
main.go — это единственное место, которое знает обо всех слоях сразу.

Dependency Injection (DI): Мы создаем объекты зависимостей и передаем их внутрь конструкторов.

Store ➔ отдаем в Service.

Service ➔ отдаем в Endpoint.

Endpoint ➔ отдаем в Transport.

Композиция (Composition Root): Здесь мы "надеваем" middleware друг на друга.

```go
// 1. Создаем ядро (Service + Business Logs)
// Сначала логика, сверху бизнес-логи
svc = newBasicService()
svc = newLoggingMiddleware(logger)(svc) 

// 2. Оборачиваем в инфраструктуру (Endpoint + RateLimit/Breaker)
// Сверху надеваем защиту
ends := aggendpoint.New(svc, logger)

// 3. Выставляем наружу (Transport + HTTP)
// Самая внешняя оболочка — протокол
handler := aggtransport.NewHTTPHandler(ends, logger)
```

Если ты захочешь поменять базу данных или перейти с HTTP на gRPC — ты меняешь код только в main.go (пересобираешь конструктор), не трогая логику внутри сервиса.