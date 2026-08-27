# Telegram News Mini App

MVP đọc RSS, lưu SQLite và hiển thị bằng Telegram Mini App. Một Go process phục vụ API, React frontend, RSS worker và Telegram bot polling; không cần Redis, PostgreSQL hay service phụ.

## Chạy bằng Docker

1. Tạo file cấu hình: `cp .env.example .env`.
2. Điền `MINI_APP_URL` (URL HTTPS Cloudflare Tunnel) và `TELEGRAM_BOT_TOKEN` khi đã có bot.
3. Chạy: `docker compose up -d --build`.
4. Kiểm tra: `curl http://localhost:8080/health` và `docker compose logs -f`.

SQLite được lưu trong named volume Docker tại `/data/news.db`. Để xem trực tiếp database: `docker compose exec news-app /bin/sh` (runtime image không cài sqlite CLI; có thể mount volume hoặc dùng tool SQLite bên ngoài).

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
cloudflared tunnel --url http://localhost:8080
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

## API

| Method | Endpoint | Mô tả |
| --- | --- | --- |
| GET | `/health` | Trạng thái service/database |
| GET | `/api/articles?category=technology&q=bitcoin&limit=20` | Feed mới nhất |
| GET | `/api/articles/:id` | Chi tiết bài |
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

## AI summary và dịch tiếng Việt

AI chỉ được cấu hình qua `.env`, không có API/UI để đọc hoặc sửa key:

```env
AI_URL=https://api.openai.com/v1/chat/completions
AI_KEY=your-server-side-api-key
AI_MODEL=gpt-4o-mini
```

`AI_URL` dùng chuẩn OpenAI-compatible Chat Completions. Khi có đủ `AI_URL` và `AI_KEY`, RSS worker gửi nội dung của **bài mới** đến AI để tạo summary tiếng Việt tối đa hai câu và lưu vào cột `articles.summary`. Nếu AI không cấu hình hoặc request lỗi, app vẫn lưu bài với summary gốc từ RSS; không một lỗi AI nào làm worker dừng. Thay đổi `.env` cần restart container: `docker compose up -d --force-recreate`.
