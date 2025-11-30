# Product Requirements Document: IPFS Content Indexer

## 1. Обзор продукта

### 1.1 Цель
Создать сервис для автоматической индексации IPFS контента, который получает анонсы коллекций через PubSub, скачивает и парсит их содержимое, создавая поисковый индекс для дальнейшего использования.

### 1.2 Scope
**В рамках текущей версии:**
- Подписка на PubSub топик и обработка анонсов коллекций
- Базовая валидация формата сообщений
- Скачивание коллекций по IPNS ссылкам с retry механизмом
- Сохранение данных в SQLite базу
- Интеграция с Quickwit для полнотекстового поиска
- Запуск embedded IPFS Cubo ноды

**Out of scope (будущие версии):**
- Валидация IPNS подписей
- Валидация контента (проверка доступности CID в сети)
- Рейтинговая система для публикаторов
- Rate limiting на публикации
- Дедупликация контента

---

## 2. Техническая архитектура

### 2.1 Компоненты системы

```
┌─────────────────┐
│   IPFS PubSub   │
│     Topic       │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────┐
│   Indexer Service           │
│  ┌─────────────────────┐    │
│  │ Embedded IPFS Node  │    │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │ PubSub Listener     │    │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │ Collection Fetcher  │    │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │ Parser & Validator  │    │
│  └─────────────────────┘    │
└──────────┬──────────────────┘
           │
           ▼
    ┌──────────────┐
    │  SQLite DB   │
    └──────┬───────┘
           │
           ▼
    ┌──────────────┐
    │  Quickwit    │
    │  (Search)    │
    └──────────────┘
```

### 2.2 IPFS Embedded Node
Запуск Cubo ноды внутри приложения со следующими параметрами:

```yaml
ipfs:
  mode: "embedded"
  embedded:
    repo_path: "./ipfs_publisher/ipfs-repo"
    swarm_port: 4002
    api_port: 5002
    gateway_port: 8081
    bootstrap_peers: []
    gc:
      enabled: true
      interval: 86400  # 24 hours
      min_free_space: 1073741824  # 1GB
```

---

## 3. Функциональные требования

### 3.1 PubSub Listener

**FR-1.1:** Сервис должен подписаться на PubSub топик, указанный в конфигурации при запуске.

**FR-1.2:** При получении сообщения извлечь следующие поля:
```json
{
  "version": 1,
  "ipns": "k2k4r8ltgwjllr3n1on4rwis0kc853wzdcyjgt5xk2lcui5xn95c5vl2",
  "publicKey": "E8WtP2ctD8iOoZ1s95xrU55a4iYaCdlUD+auyMZfPLM=",
  "collectionSize": 4,
  "timestamp": 1764260509,
  "signature": "XoFDGnjThpqJnmh0/c8nERCOxNjly20007VqZAqpaUnZ5m5VGsIUjIBFYu/W62c5IQ4qDaM5ysHQJVK7jkAyAg=="
}
```

**FR-1.3:** Извлечь public key отправителя сообщения из метаданных PubSub (host public key).

**FR-1.4:** Базовая валидация:
- Проверить наличие обязательных полей: `version`, `ipns`, `publicKey`, `timestamp`
- Проверить корректность формата IPNS (начинается с "k2k4r8")
- Если валидация не прошла - логировать ошибку и пропустить сообщение

### 3.2 Database Management

**FR-2.1:** Использовать SQLite как основную БД с возможностью расширения под другие СУБД через интерфейс.

**FR-2.2:** Структура базы данных:

```sql
-- Таблица хостов (IPFS ноды, отправившие сообщение в PubSub)
CREATE TABLE hosts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    public_key TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица публикаторов (владельцы IPNS ключей)
CREATE TABLE publishers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    public_key TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица коллекций
CREATE TABLE collections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id INTEGER NOT NULL,
    publisher_id INTEGER NOT NULL,
    version INTEGER NOT NULL,
    ipns TEXT NOT NULL,
    size INTEGER,
    timestamp INTEGER NOT NULL,
    status TEXT DEFAULT 'pending', -- pending, downloaded, failed
    retry_count INTEGER DEFAULT 0,
    last_retry_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (host_id) REFERENCES hosts(id),
    FOREIGN KEY (publisher_id) REFERENCES publishers(id)
);

-- Индекс контента
CREATE TABLE index_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cid TEXT NOT NULL,
    filename TEXT NOT NULL,
    extension TEXT NOT NULL,
    host_id INTEGER NOT NULL,
    publisher_id INTEGER NOT NULL,
    collection_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (host_id) REFERENCES hosts(id),
    FOREIGN KEY (publisher_id) REFERENCES publishers(id),
    FOREIGN KEY (collection_id) REFERENCES collections(id)
);

-- Индекс для быстрого поиска по CID и коллекции
CREATE INDEX idx_index_items_cid ON index_items(cid);
CREATE INDEX idx_index_items_collection ON index_items(collection_id);
CREATE INDEX idx_collections_ipns_version ON collections(ipns, version);
```

**FR-2.3:** При получении анонса:
1. Создать/найти запись в `hosts` по public key отправителя
2. Создать/найти запись в `publishers` по public key из сообщения
3. Создать запись в `collections` со статусом `pending`

### 3.3 Collection Fetcher

**FR-3.1:** Для каждой коллекции со статусом `pending`:
1. Сформировать путь `/ipns/{ipns_value}`
2. Запросить файл через embedded IPFS ноду
3. Обработать файл или перейти к retry логике

**FR-3.2:** Retry механизм:
- При недоступности IPNS ссылки повторить попытку через 1 минуту
- Максимум 10 попыток (всего 10 минут ожидания)
- После каждой попытки обновить `retry_count` и `last_retry_at`
- После 10 неудачных попыток установить статус `failed`

**FR-3.3:** При успешном скачивании:
- Установить статус `downloaded`
- Обновить `size` коллекции (если доступно)
- Передать содержимое в Parser

### 3.4 Parser & Storage

**FR-4.1:** Формат файла коллекции - JSONL (JSON Lines):
```
{"id":2,"CID":"QmaYsXFBVpMMk74Ed78342XSH26wQZs9Y8PyAUWNCxzyZp","filename":"test-15mb.mp3","extension":"mp3"}
{"id":7,"CID":"QmepHP9vMsBZB7w15yEqnUzTupNoQqnG9Lj3VhBQAvxg6B","filename":"Olga_Buzova.mp3","extension":"mp3"}
```

**FR-4.2:** Для каждой строки файла:
1. Распарсить JSON
2. Проверить наличие обязательных полей: `CID`, `filename`, `extension`
3. Пропустить строку при отсутствии обязательных полей (логировать предупреждение)

**FR-4.3:** Логика обработки при новой версии коллекции:
- Если запись с таким же `CID` + `collection_id` уже существует - обновить `filename`, `extension`, `updated_at`
- Если записи нет - создать новую запись в `index_items`
- Старые записи (которых нет в новой версии) НЕ удаляются

**FR-4.4:** Генерировать собственный `id` для каждой записи в `index_items` (игнорировать `id` из файла коллекции)

### 3.5 Quickwit Integration

**FR-5.1:** Конфигурация индекса Quickwit:
```yaml
version: 0.7
index_id: ipfs_content

doc_mapping:
  field_mappings:
    - name: id
      type: i64
      indexed: true
    - name: cid
      type: text
      tokenizer: default
      indexed: true
    - name: filename
      type: text
      tokenizer: default
      indexed: true
    - name: extension
      type: text
      tokenizer: raw
      indexed: true
    - name: timestamp
      type: datetime
      indexed: true

indexing_settings:
  commit_timeout_secs: 60
```

**FR-5.2:** Батчинг:
- Накапливать записи в буфере размером 1000 записей или 30 секунд (что наступит раньше)
- Отправить batch в Quickwit через REST API
- При ошибке отправки логировать и повторить через 10 секунд (до 3 попыток)

**FR-5.3:** Синхронизация:
- Отправлять в Quickwit только данные из `index_items` без JOIN'ов
- Добавлять `timestamp` из `collections.timestamp` для каждой записи

---

## 4. Конфигурация

### 4.1 Структура конфигурационного файла

```yaml
# config.yaml

# Database settings
database:
  type: "sqlite"  # sqlite, postgres, mysql
  path: "./data/indexer.db"  # for SQLite
  # connection_string: "..."  # for other DB types

# IPFS settings
ipfs:
  mode: "embedded"
  embedded:
    repo_path: "./ipfs_publisher/ipfs-repo"
    swarm_port: 4002
    api_port: 5002
    gateway_port: 8081
    bootstrap_peers: []
    gc:
      enabled: true
      interval: 86400
      min_free_space: 1073741824

# PubSub settings
pubsub:
  topic: "ipfs-collections-index"  # configurable topic name

# Fetcher settings
fetcher:
  retry_attempts: 10
  retry_interval_seconds: 60
  concurrent_downloads: 5  # max parallel IPNS fetches

# Quickwit settings
quickwit:
  url: "http://localhost:7280"
  index_id: "ipfs_content"
  batch_size: 1000
  batch_timeout_seconds: 30
  retry_attempts: 3
  retry_interval_seconds: 10

# Logging
logging:
  level: "info"  # debug, info, warn, error
  format: "text"  # text, json
  output: "stdout"  # stdout, file
  file_path: "./logs/indexer.log"  # if output: file
```

---

## 5. Нефункциональные требования

### 5.1 Производительность
- **NFR-1.1:** Обработка одного анонса коллекции не должна блокировать обработку других анонсов
- **NFR-1.2:** Поддержка параллельного скачивания до 5 коллекций одновременно (configurable)

### 5.2 Надежность
- **NFR-2.1:** При сбое приложения не должна теряться информация о необработанных коллекциях
- **NFR-2.2:** Graceful shutdown - завершить текущие операции перед остановкой

### 5.3 Масштабируемость
- **NFR-3.1:** Архитектура должна позволять замену SQLite на PostgreSQL/MySQL без изменения бизнес-логики
- **NFR-3.2:** Возможность добавления нескольких PubSub топиков в будущем

### 5.4 Логирование
- **NFR-4.1:** Базовые логи для всех операций:
  - Получение PubSub сообщения (INFO)
  - Начало/завершение скачивания коллекции (INFO)
  - Ошибки валидации (WARN)
  - Ошибки скачивания/парсинга (ERROR)
  - Статистика батча в Quickwit (INFO)

---

## 6. API и интерфейсы

### 6.1 Внешние интерфейсы
- **Quickwit UI:** Базовый интерфейс для полнотекстового поиска по индексу
  - URL: `http://localhost:7280/ui/search?index=ipfs_content`
  - Доступные поля для поиска: `cid`, `filename`, `extension`

### 6.2 Внутренние интерфейсы
Нет требований к REST API в текущей версии. Вся работа происходит автоматически через PubSub.

---

## 7. Сценарии использования

### 7.1 UC-1: Обработка нового анонса
1. Публикатор отправляет анонс в PubSub топик
2. Indexer получает сообщение и валидирует его
3. Создаются записи в `hosts`, `publishers`, `collections`
4. Запускается асинхронное скачивание по IPNS ссылке
5. Файл парсится и записывается в `index_items`
6. Данные батчами отправляются в Quickwit
7. Контент становится доступен для поиска

### 7.2 UC-2: Обработка недоступной IPNS ссылки
1. Indexer пытается скачать коллекцию, но ссылка недоступна
2. Коллекция остается в статусе `pending`
3. Через 1 минуту повторяется попытка (до 10 раз)
4. После 10 неудач статус меняется на `failed`
5. Коллекция больше не обрабатывается

### 7.3 UC-3: Обновление существующей коллекции
1. Приходит анонс с той же IPNS, но большей версией
2. Скачивается новая версия файла
3. Для совпадающих CID обновляются `filename` и `extension`
4. Новые CID добавляются в индекс
5. Старые записи остаются без изменений
6. Обновления синхронизируются с Quickwit

### 7.4 UC-4: Поиск контента
1. Пользователь открывает Quickwit UI
2. Вводит поисковый запрос (например, "batman")
3. Получает результаты с полями: `id`, `cid`, `filename`, `extension`
4. Может использовать CID для скачивания через IPFS

---

## 8. Критерии успеха

### 8.1 MVP критерии
- ✅ Приложение успешно подписывается на PubSub топик
- ✅ Обрабатывает минимум 100 анонсов без сбоев
- ✅ Корректно скачивает и парсит коллекции по IPNS
- ✅ Сохраняет данные в SQLite
- ✅ Индексирует контент в Quickwit
- ✅ Поиск в Quickwit UI возвращает корректные результаты
- ✅ Retry механизм работает для недоступных ссылок

### 8.2 Метрики качества
- Успешная обработка >95% валидных анонсов
- Среднее время от получения анонса до индексации <5 минут (при доступной IPNS)

---

## 9. Риски и ограничения

### 9.1 Технические риски
- **R-1:** IPNS ссылки могут быть медленными или недоступными
  - *Митигация:* Retry механизм с достаточным таймаутом
  
- **R-2:** Большой размер коллекций может замедлить обработку
  - *Митигация:* Асинхронная обработка и параллельное скачивание

- **R-3:** SQLite может стать узким местом при больших объемах
  - *Митигация:* Подготовка интерфейса для миграции на PostgreSQL

### 9.2 Ограничения текущей версии
- Нет валидации подписей и контента
- Нет защиты от спама/вредоносных публикаций
- Нет механизма проверки доступности CID в сети
- Нет REST API для внешнего доступа
- Нет дедупликации одинаковых CID от разных публикаторов

---

## 10. Roadmap (Future Scope)

### Phase 2 (будущие версии)
- Валидация IPNS подписей
- Background валидация контента (routing findprovs)
- Rate limiting на публикации от хостов
- Рейтинговая система публикаторов
- REST API для программного доступа
- Обновление метаданных из публичных баз (MusicBrainz, TMDB)
- Поддержка обложек альбомов

### Phase 3 (long-term)
- Распределенная архитектура с несколькими индексерами
- Machine learning для детекции вредоносного контента
- Интеграция с другими децентрализованными хранилищами
- Продвинутая аналитика и статистика

---

## Приложение A: Примеры данных

### A.1 Пример PubSub сообщения
```json
{
  "version": 1,
  "ipns": "k2k4r8ltgwjllr3n1on4rwis0kc853wzdcyjgt5xk2lcui5xn95c5vl2",
  "publicKey": "E8WtP2ctD8iOoZ1s95xrU55a4iYaCdlUD+auyMZfPLM=",
  "collectionSize": 4,
  "timestamp": 1764260509,
  "signature": "XoFDGnjThpqJnmh0/c8nERCOxNjly20007VqZAqpaUnZ5m5VGsIUjIBFYu/W62c5IQ4qDaM5ysHQJVK7jkAyAg=="
}
```

### A.2 Пример файла коллекции (JSONL)
```
{"id":2,"CID":"QmaYsXFBVpMMk74Ed78342XSH26wQZs9Y8PyAUWNCxzyZp","filename":"test-15mb.mp3","extension":"mp3"}
{"id":7,"CID":"QmepHP9vMsBZB7w15yEqnUzTupNoQqnG9Lj3VhBQAvxg6B","filename":"Olga_Buzova_i_Lyosha_Svik_-_Poceluj_na_balkone.mp3","extension":"mp3"}
{"id":8,"CID":"QmTDWHWuNoVK1pVPooLWsjUEjaYwRRwgmN22prRFd5yyPF","filename":"Prodigy_-_Smak_My_Bitch_Up.mp3","extension":"mp3"}
{"id":9,"CID":"QmQPAXaxckNS8oENgxCi26VewE7ac7kV5c6WXhjnuWbMbc","filename":"file_example_AVI_1280_1_5MG.avi","extension":"avi"}
{"id":10,"CID":"bafkreid3cyrzhkewyf6pd4eqb2ughbaxtokpuwi7xeabgxk46yo6qerwya","filename":"winamp-it-really-whips-the-llamas-ass.mp3","extension":"mp3"}
```

### A.3 Пример записи в базе (index_items)
```sql
id: 1
cid: "QmaYsXFBVpMMk74Ed78342XSH26wQZs9Y8PyAUWNCxzyZp"
filename: "test-15mb.mp3"
extension: "mp3"
host_id: 1
publisher_id: 1
collection_id: 1
created_at: "2024-11-27 10:30:00"
updated_at: "2024-11-27 10:30:00"
```

---

## Версионирование документа
- **v1.0** - 2024-11-27 - Первая версия PRD (базовая функциональность)