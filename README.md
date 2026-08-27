# Telegram News Mini App

MVP đọc RSS, lưu SQLite và hiển thị bằng Telegram Mini App. Một Go process phục vụ API, React frontend, RSS worker và Telegram bot polling; không cần Redis, PostgreSQL hay service phụ.

## Cấu trúc backend

`backend/cmd/server/` vẫn build thành một binary duy nhất, nhưng đã tách các concern nền tảng theo file:

- `main.go`: khởi tạo server, shutdown và phần HTTP/RSS/Bot wiring.
- `config.go`: đọc và parse toàn bộ biến môi trường.
- `database.go`: SQLite connection, pragmas, migration và RSS seed.
- `models.go`: DTO và model nội bộ dùng chung.
- `migrations/` và `static/`: tài nguyên được embed vào binary; Docker copy React build vào `static/` trước khi compile.
- `internal/crawlers/<source>/`: cấu hình và parser riêng theo từng nguồn; hiện SCMP RSS nằm ở `internal/crawlers/scmp/`.
- `internal/crawlers/rss/`: contract `rss.Source` chung; mọi site RSS chỉ khai báo `Name`, `URL`, `Category`, sau đó dùng chung worker fetch/parse/dedupe.

## Chạy bằng Docker

1. Tạo file cấu hình: `cp .env.example .env`.
2. Điền `MINI_APP_URL` (URL HTTPS Cloudflare Tunnel) và `TELEGRAM_BOT_TOKEN` khi đã có bot.
3. Chạy: `make restart` (tương đương `docker compose up -d --build --force-recreate`).
4. Kiểm tra: `curl http://localhost:8080/health` và `docker compose logs -f`.

SQLite được bind mount trực tiếp tại `./data/news.db` trong project (tương ứng `/data/news.db` trong container). Bạn có thể mở file này bằng SQLite client trên máy host.

Các lệnh thường dùng: `make build` để build image, `make up` để start, `make restart` để build lại và restart app, `make logs` để xem log, và `make down` để dừng app.

Nếu chưa có bot/token, app vẫn chạy và RSS tự tải tin; đặt `DEV_AUTH=true` chỉ để thử Saved trong trình duyệt local. Không bật biến này ở môi trường public.

## Tạo Telegram Bot và Mini App

1. Mở [@BotFather](https://t.me/BotFather), dùng `/newbot`, rồi lưu token vào `TELEGRAM_BOT_TOKEN`.
2. Deploy app sau reverse proxy HTTPS (ngrok/Cloudflare Tunnel cho thử nghiệm), rồi đặt URL đó vào `MINI_APP_URL`.
3. Trong BotFather, đặt menu button/Mini App trỏ tới cùng URL. Khi user gửi `/start`, bot cũng trả một nút **Open News**.
4. Mở Mini App từ Telegram. Frontend gửi `Telegram.WebApp.initData` cho backend; backend xác thực chữ ký bằng bot token. Bot token không bao giờ đi vào bundle frontend.

Telegram yêu cầu HTTPS cho Mini App production. Browser ở `localhost` không sinh `initData`; dùng `DEV_AUTH=true` cho UI local là hợp lệ duy nhất cho development.

### Dùng Cloudflare Tunnel

Sau khi container đang chạy ở cổng 8080, mở một terminal khác và chạy Quick Tunnel:

```bash
cloudflared tunnel --url http://localhost:1999
```
```bash
nohup cloudflared tunnel --url http://localhost:8080 > ~/cloudflared.log 2>&1 &
  echo $!
```

Lệnh in ra một URL dạng `https://news-abc.trycloudflare.com`. Gán URL đó vào `MINI_APP_URL` trong `.env`, sau đó restart app để bot dùng URL mới:

```bash
docker compose up -d --force-recreate
```

Đặt cùng URL trong BotFather cho Menu Button/Mini App. Quick Tunnel đổi URL sau mỗi lần khởi động; khi dùng lâu dài, hãy tạo named tunnel trong Cloudflare Zero Trust với hostname cố định, rồi đặt hostname đó vào `MINI_APP_URL`.

## Chạy local

Cần Go 1.24+, Node 22+ và pnpm 10+ (Node 22 có thể chạy `corepack enable` để cài pnpm).

```bash
cp .env.example .env
mkdir -p data
cd frontend && pnpm install && pnpm dev
# Terminal khác
cd backend && DATABASE_PATH=../data/news.db DEV_AUTH=true go run ./cmd/server
```

Vite mặc định chạy cổng 5173 và proxy `/api` về Go ở 8080, nên feed/Saved chạy được khi mở `http://localhost:5173`. Để Go tự phục vụ frontend như production, build frontend (`pnpm build`) rồi copy `frontend/dist` thành `backend/static`; Docker tự làm các bước này.

Mỗi bài có URL trực tiếp dạng `/news/:id-slug`, ví dụ `/news/156-a-news-headline`. Mở trực tiếp URL này vẫn tải đúng trang chi tiết.

## API

| Method | Endpoint | Mô tả |
| --- | --- | --- |
| GET | `/health` | Trạng thái service/database |
| GET | `/api/articles?category=technology&q=bitcoin&limit=20` | Feed mới nhất |
| GET | `/api/articles/:id` | Chi tiết bài |
| POST | `/api/articles/:id/translations/vi` | Dịch và lấy cache tiếng Việt (Telegram auth) |
| GET | `/api/categories` | Danh sách category |
| GET | `/api/saved` | Bài đã lưu (Telegram auth) |
| POST/DELETE | `/api/saved/:id` | Lưu/bỏ lưu (Telegram auth) |

Các API Saved nhận `Authorization: tma <Telegram initData>`. Backend kiểm tra HMAC Telegram, `auth_date` (mặc định tối đa 24 giờ) và upsert user trước khi truy cập dữ liệu.

## Thêm RSS source

Ba nguồn mẫu được seed trong `backend/cmd/server/main.go`: Hacker News, BBC World và BBC Business. Thêm một entry vào danh sách `seeds` trong hàm `migrate`, build lại image và restart. Với database đang chạy, cũng có thể thêm bằng SQL:

```sql
INSERT INTO sources (name, feed_url, category_id)
VALUES ('Example Tech', 'https://example.com/feed.xml',
        (SELECT id FROM categories WHERE slug = 'technology'));
```

Worker chạy lúc khởi động và mỗi `RSS_FETCH_INTERVAL` (mặc định `10m`). URL là unique nên bài cũ không được chèn lại; lỗi một feed chỉ được log và không ảnh hưởng app hoặc feed còn lại.

Với mỗi bài RSS mới, worker tạo một bản tóm tắt AI và giữ link về bài gốc. Có thể đặt `RSS_CONTENT_USER_AGENT` để nhận diện request lấy ngữ cảnh cho việc tóm tắt.

RSS tổng hợp SCMP `https://www.scmp.com/rss/feed` được seed mặc định, để lấy tin mới từ toàn site. Có thể thêm feed theo category trên [SCMP RSS](https://www.scmp.com/rss); bài xuất hiện ở nhiều feed vẫn chỉ được lưu một lần nhờ unique URL.

Chạy thủ công một lượt RSS SCMP (không start HTTP server, Telegram Bot hay scheduler):

```bash
docker compose build news-app
docker compose run --rm --no-deps news-app /app/news rss-fetch scmp
```

Để fetch toàn bộ RSS source đang bật:

```bash
docker compose run --rm --no-deps news-app /app/news rss-fetch
```

Chỉ lấy các bài trong số ngày gần đây (1–30 ngày):

```bash
docker compose run --rm --no-deps news-app /app/news rss-fetch --days 7
```

Hoặc lấy bài có ngày xuất bản từ một ngày cụ thể (UTC):

```bash
docker compose run --rm --no-deps news-app /app/news rss-fetch --since 2026-08-20
```

Có thể kết hợp filter nguồn và ngày, ví dụ: `docker compose run --rm --no-deps news-app /app/news rss-fetch bbc --days 3`.

## Dịch tiếng Việt theo yêu cầu

Để dịch và cache tiếng Việt cho toàn bộ bài chưa có bản dịch (chỉ title/tóm tắt), chạy:

```bash
docker compose run --rm --no-deps news-app /app/news translate-all-vi
```

AI chỉ được cấu hình qua `.env`, không có API/UI để đọc hoặc sửa key:

```env
AI_URL=https://api.openai.com/v1/chat/completions
AI_KEY=your-server-side-api-key
AI_MODEL=gpt-4o-mini
```

`AI_URL` dùng chuẩn OpenAI-compatible Chat Completions. RSS tạo tóm tắt khi nhập bài; bản dịch title và tóm tắt được lưu trong SQLite theo bài/ngôn ngữ và dùng lại từ cache. Nếu AI chưa cấu hình hoặc trả lỗi, app giữ metadata RSS và link bài gốc. Thay đổi `.env` cần restart container: `docker compose up -d --force-recreate`.

## Tạo bản tin nổi bật thủ công

Khi `AI_URL` và `AI_KEY` đã được cấu hình, tạo ngay một bản tin nổi bật từ các bài đủ điều kiện trong cửa sổ `FEATURED_BRIEF_WINDOW`:

```bash
docker compose run --rm --no-deps news-app /app/news featured-generate
```

Lệnh sẽ ghi log nếu thiếu cấu hình AI, chưa đủ 5 bài đã tóm tắt, hoặc bản tin đã được tạo cho slot thời gian hiện tại.
