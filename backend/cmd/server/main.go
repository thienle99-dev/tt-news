package main

import (
 "context"
 "crypto/hmac"
 "crypto/sha256"
 "database/sql"
 "embed"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "io/fs"
 "log"
 "net/http"
 "net/url"
 "os"
 "os/signal"
 "path"
 "sort"
 "strconv"
 "strings"
 "syscall"
 "time"

 "github.com/go-chi/chi/v5"
 "github.com/mmcdole/gofeed"
 _ "modernc.org/sqlite"
)

//go:embed migrations/001_init.sql static/*
var embedded embed.FS

type config struct { Port, DBPath, BotToken, MiniAppURL, AIURL, AIKey, AIModel string; RSSInterval, AuthMaxAge time.Duration; DevAuth bool; DevUserID int64 }
type user struct { ID int64; TelegramID int64 `json:"telegram_id"`; Username string `json:"username"`; FirstName string `json:"first_name"`; LastName string `json:"last_name"`; PhotoURL string `json:"photo_url"` }
type source struct { ID int64; Name, FeedURL string; CategoryID int64; Category string }
type article struct { ID int64 `json:"id"`; Title string `json:"title"`; Description string `json:"description"`; Summary string `json:"summary"`; URL string `json:"url"`; ImageURL string `json:"image_url"`; Source string `json:"source"`; Category string `json:"category"`; PublishedAt string `json:"published_at"`; IsSaved bool `json:"is_saved"` }
type ctxKey string
const userKey ctxKey = "user"

func main() {
 cfg := loadConfig()
 if len(os.Args) > 1 && os.Args[1] == "healthcheck" { if health(cfg.DBPath) { return }; os.Exit(1) }
 db, err := openDB(cfg.DBPath); if err != nil { log.Fatal(err) }; defer db.Close()
 if err = migrate(db); err != nil { log.Fatal(err) }
 app := &server{db: db, cfg: cfg, feed: gofeed.NewParser()}
 ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer cancel()
 go app.runRSS(ctx)
 if cfg.BotToken != "" && cfg.MiniAppURL != "" { go app.runBot(ctx) } else { log.Print("Telegram bot disabled: set TELEGRAM_BOT_TOKEN and MINI_APP_URL to enable it") }
 h := app.routes()
 httpServer := &http.Server{Addr: ":" + cfg.Port, Handler: h, ReadHeaderTimeout: 10*time.Second}
 go func(){ <-ctx.Done(); c, stop := context.WithTimeout(context.Background(), 10*time.Second); defer stop(); _ = httpServer.Shutdown(c) }()
 log.Printf("news app listening on :%s", cfg.Port)
 if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err,http.ErrServerClosed) { log.Fatal(err) }
}

func loadConfig() config { return config{Port: env("PORT","8080"), DBPath: env("DATABASE_PATH","/data/news.db"), BotToken: os.Getenv("TELEGRAM_BOT_TOKEN"), MiniAppURL: os.Getenv("MINI_APP_URL"), AIURL: os.Getenv("AI_URL"), AIKey: os.Getenv("AI_KEY"), AIModel: env("AI_MODEL","gpt-4o-mini"), RSSInterval: duration("RSS_FETCH_INTERVAL",10*time.Minute), AuthMaxAge: duration("TELEGRAM_AUTH_MAX_AGE",24*time.Hour), DevAuth: env("DEV_AUTH","false")=="true", DevUserID: intEnv("DEV_USER_ID",999001)} }
func env(k,d string) string { if v:=os.Getenv(k); v!="" { return v }; return d }
func duration(k string,d time.Duration) time.Duration { if v,e:=time.ParseDuration(env(k,"")); e==nil && v>0{return v}; return d }
func intEnv(k string,d int64) int64 { if v,e:=strconv.ParseInt(env(k,""),10,64);e==nil{return v};return d }
func health(dbPath string) bool { db,e:=openDB(dbPath); if e!=nil{return false}; defer db.Close(); return db.Ping()==nil }
func openDB(file string)(*sql.DB,error){ db,e:=sql.Open("sqlite",file); if e!=nil{return nil,e}; db.SetMaxOpenConns(1); for _,q:=range []string{"PRAGMA journal_mode=WAL","PRAGMA foreign_keys=ON","PRAGMA busy_timeout=5000"}{if _,e=db.Exec(q);e!=nil{db.Close();return nil,e}};return db,nil }
func migrate(db *sql.DB) error { b,e:=embedded.ReadFile("migrations/001_init.sql");if e!=nil{return e};if _,e=db.Exec(string(b));e!=nil{return e};if _,e=db.Exec("ALTER TABLE articles ADD COLUMN summary TEXT NOT NULL DEFAULT ''");e!=nil&&!strings.Contains(e.Error(),"duplicate column name"){return e}; cats:=[]struct{slug,name string}{{"technology","Technology"},{"world","World"},{"business","Business"}}; for _,c:=range cats {if _,e=db.Exec("INSERT OR IGNORE INTO categories(slug,name) VALUES(?,?)",c.slug,c.name);e!=nil{return e}}; seeds:=[]struct{name,url,slug string}{{"Hacker News","https://hnrss.org/frontpage","technology"},{"BBC World","https://feeds.bbci.co.uk/news/world/rss.xml","world"},{"BBC Business","https://feeds.bbci.co.uk/news/business/rss.xml","business"}};for _,s:=range seeds{if _,e=db.Exec("INSERT OR IGNORE INTO sources(name,feed_url,category_id) VALUES(?,?,(SELECT id FROM categories WHERE slug=?))",s.name,s.url,s.slug);e!=nil{return e}};return nil }

type server struct { db *sql.DB; cfg config; feed *gofeed.Parser }
func (s *server) routes() http.Handler { r:=chi.NewRouter(); r.Use(recoverer); r.Get("/health",func(w http.ResponseWriter,r *http.Request){jsonOut(w,200,map[string]string{"status":"ok"})}); r.Route("/api",func(r chi.Router){r.Use(jsonContent);r.Get("/articles",s.listArticles);r.Get("/articles/{id}",s.getArticle);r.Get("/categories",s.categories);r.Group(func(r chi.Router){r.Use(s.requireUser);r.Get("/saved",s.saved);r.Post("/saved/{id}",s.save);r.Delete("/saved/{id}",s.unsave);r.Get("/me",s.me)})}); sub,_:=fs.Sub(embedded,"static");r.Handle("/*",spa(sub));return r }
func recoverer(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){defer func(){if recover()!=nil{jsonErr(w,500,"internal server error")}}();next.ServeHTTP(w,r)})}
func jsonContent(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json; charset=utf-8");next.ServeHTTP(w,r)})}
func jsonOut(w http.ResponseWriter,status int,v any){w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func jsonErr(w http.ResponseWriter,status int,msg string){jsonOut(w,status,map[string]string{"error":msg})}
func spa(files fs.FS) http.Handler { fileServer:=http.FileServer(http.FS(files));return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){p:=strings.TrimPrefix(path.Clean(r.URL.Path),"/");if p=="."{p=""};if p!=""{if f,e:=files.Open(p);e==nil{f.Close();fileServer.ServeHTTP(w,r);return}};r.URL.Path="/";fileServer.ServeHTTP(w,r)}) }

func (s *server) optionalUser(r *http.Request)(user,bool){ raw:=strings.TrimPrefix(r.Header.Get("Authorization"),"tma "); if raw!="" {u,e:=verifyInitData(raw,s.cfg.BotToken,s.cfg.AuthMaxAge);if e==nil{u,e=s.upsertUser(r.Context(),u);if e==nil{return u,true};log.Printf("user upsert: %v",e)}};return user{},false }
func (s *server) requireUser(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){u,ok:=s.optionalUser(r);if !ok && s.cfg.DevAuth {var e error;u,e=s.upsertUser(r.Context(),user{TelegramID:s.cfg.DevUserID,FirstName:"Local developer",Username:"dev"});ok=e==nil};if !ok{jsonErr(w,401,"valid Telegram initData is required");return};next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),userKey,u)))})}
func currentUser(r *http.Request) user {u,_:=r.Context().Value(userKey).(user);return u}
func (s *server) upsertUser(ctx context.Context,u user)(user,error){_,e:=s.db.ExecContext(ctx,`INSERT INTO users(telegram_id,username,first_name,last_name,photo_url) VALUES(?,?,?,?,?) ON CONFLICT(telegram_id) DO UPDATE SET username=excluded.username,first_name=excluded.first_name,last_name=excluded.last_name,photo_url=excluded.photo_url,updated_at=CURRENT_TIMESTAMP`,u.TelegramID,u.Username,u.FirstName,u.LastName,u.PhotoURL);if e!=nil{return u,e};e=s.db.QueryRowContext(ctx,"SELECT id FROM users WHERE telegram_id=?",u.TelegramID).Scan(&u.ID);return u,e}
func (s *server) listArticles(w http.ResponseWriter,r *http.Request){q:=r.URL.Query();limit:=intEnvFrom(q.Get("limit"),20);if limit<1{limit=20};if limit>100{limit=100};u,logged:=s.optionalUser(r);savedJoin:="0";args:=[]any{};if logged{savedJoin="EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.article_id=a.id AND sa.user_id=?)";args=append(args,u.ID)};where:=[]string{"1=1"};if c:=q.Get("category");c!=""{where=append(where,"c.slug=?");args=append(args,c)};if term:=strings.TrimSpace(q.Get("q"));term!=""{where=append(where,"(a.title LIKE ? OR a.description LIKE ? OR a.summary LIKE ?)");like:="%"+term+"%";args=append(args,like,like,like)};args=append(args,limit);sqlq:=fmt.Sprintf(`SELECT a.id,a.title,a.description,a.summary,a.url,a.image_url,s.name,c.slug,a.published_at,%s FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE %s ORDER BY a.published_at DESC,a.id DESC LIMIT ?`,savedJoin,strings.Join(where," AND "));rows,e:=s.db.QueryContext(r.Context(),sqlq,args...);if e!=nil{jsonErr(w,500,"could not load articles");return};defer rows.Close();items:=[]article{};for rows.Next(){var a article;var saved int;if e=rows.Scan(&a.ID,&a.Title,&a.Description,&a.Summary,&a.URL,&a.ImageURL,&a.Source,&a.Category,&a.PublishedAt,&saved);e!=nil{jsonErr(w,500,"could not read articles");return};a.IsSaved=saved==1;items=append(items,a)};jsonOut(w,200,items)}
func (s *server) getArticle(w http.ResponseWriter,r *http.Request){id:=chi.URLParam(r,"id");var a article;e:=s.db.QueryRowContext(r.Context(),`SELECT a.id,a.title,a.description,a.summary,a.url,a.image_url,s.name,c.slug,a.published_at,0 FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE a.id=?`,id).Scan(&a.ID,&a.Title,&a.Description,&a.Summary,&a.URL,&a.ImageURL,&a.Source,&a.Category,&a.PublishedAt,new(int));if errors.Is(e,sql.ErrNoRows){jsonErr(w,404,"article not found");return};if e!=nil{jsonErr(w,500,"could not load article");return};jsonOut(w,200,a)}
func (s *server) categories(w http.ResponseWriter,r *http.Request){rows,e:=s.db.QueryContext(r.Context(),"SELECT slug,name FROM categories ORDER BY name");if e!=nil{jsonErr(w,500,"could not load categories");return};defer rows.Close();out:=[]map[string]string{};for rows.Next(){var slug,name string;_ = rows.Scan(&slug,&name);out=append(out,map[string]string{"slug":slug,"name":name})};jsonOut(w,200,out)}
func (s *server) saved(w http.ResponseWriter,r *http.Request){u:=currentUser(r);rows,e:=s.db.QueryContext(r.Context(),`SELECT a.id,a.title,a.description,a.summary,a.url,a.image_url,s.name,c.slug,a.published_at,1 FROM saved_articles sa JOIN articles a ON a.id=sa.article_id JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE sa.user_id=? ORDER BY sa.created_at DESC`,u.ID);if e!=nil{jsonErr(w,500,"could not load saved articles");return};defer rows.Close();out:=[]article{};for rows.Next(){var a article;var x int;if e=rows.Scan(&a.ID,&a.Title,&a.Description,&a.Summary,&a.URL,&a.ImageURL,&a.Source,&a.Category,&a.PublishedAt,&x);e!=nil{jsonErr(w,500,"could not read saved articles");return};a.IsSaved=true;out=append(out,a)};jsonOut(w,200,out)}
func (s *server) save(w http.ResponseWriter,r *http.Request){u:=currentUser(r);id:=chi.URLParam(r,"id");res,e:=s.db.ExecContext(r.Context(),"INSERT OR IGNORE INTO saved_articles(user_id,article_id) SELECT ?,id FROM articles WHERE id=?",u.ID,id);if e!=nil{jsonErr(w,500,"could not save article");return};n,_:=res.RowsAffected();if n==0{var exists int;e=s.db.QueryRowContext(r.Context(),"SELECT 1 FROM articles WHERE id=?",id).Scan(&exists);if errors.Is(e,sql.ErrNoRows){jsonErr(w,404,"article not found");return}};jsonOut(w,200,map[string]bool{"saved":true})}
func (s *server) unsave(w http.ResponseWriter,r *http.Request){u:=currentUser(r);_,e:=s.db.ExecContext(r.Context(),"DELETE FROM saved_articles WHERE user_id=? AND article_id=?",u.ID,chi.URLParam(r,"id"));if e!=nil{jsonErr(w,500,"could not remove saved article");return};jsonOut(w,200,map[string]bool{"saved":false})}
func (s *server) me(w http.ResponseWriter,r *http.Request){jsonOut(w,200,currentUser(r))}

func verifyInitData(raw,token string,maxAge time.Duration)(user,error){if token==""{return user{},errors.New("bot token missing")};v,e:=url.ParseQuery(raw);if e!=nil{return user{},e};hash:=v.Get("hash");authDate:=v.Get("auth_date");userJSON:=v.Get("user");if hash==""||authDate==""||userJSON==""{return user{},errors.New("missing initData fields")};v.Del("hash");keys:=make([]string,0,len(v));for k:=range v{keys=append(keys,k)};sort.Strings(keys);parts:=make([]string,0,len(keys));for _,k:=range keys{parts=append(parts,k+"="+v.Get(k))};secretMac:=hmac.New(sha256.New,[]byte("WebAppData"));secretMac.Write([]byte(token));mac:=hmac.New(sha256.New,secretMac.Sum(nil));mac.Write([]byte(strings.Join(parts,"\n")));expected:=mac.Sum(nil);provided,e:=hex.DecodeString(hash);if e!=nil||!hmac.Equal(expected,provided){return user{},errors.New("invalid initData signature")};ts,e:=strconv.ParseInt(authDate,10,64);if e!=nil||time.Since(time.Unix(ts,0))>maxAge{return user{},errors.New("expired initData")};var payload struct{ID int64 `json:"id"`;Username string `json:"username"`;FirstName string `json:"first_name"`;LastName string `json:"last_name"`;PhotoURL string `json:"photo_url"`};if e=json.Unmarshal([]byte(userJSON),&payload);e!=nil||payload.ID==0{return user{},errors.New("invalid Telegram user")};return user{TelegramID:payload.ID,Username:payload.Username,FirstName:payload.FirstName,LastName:payload.LastName,PhotoURL:payload.PhotoURL},nil}
func intEnvFrom(v string,d int)int{n,e:=strconv.Atoi(v);if e!=nil{return d};return n}

func (s *server) runRSS(ctx context.Context){s.fetchAll(ctx);t:=time.NewTicker(s.cfg.RSSInterval);defer t.Stop();for{select{case<-ctx.Done():return;case<-t.C:s.fetchAll(ctx)}}}
func (s *server) fetchAll(ctx context.Context){rows,e:=s.db.QueryContext(ctx,`SELECT s.id,s.name,s.feed_url,s.category_id,c.slug FROM sources s JOIN categories c ON c.id=s.category_id WHERE s.enabled=1`);if e!=nil{log.Printf("rss sources: %v",e);return};defer rows.Close();for rows.Next(){var src source;if e=rows.Scan(&src.ID,&src.Name,&src.FeedURL,&src.CategoryID,&src.Category);e!=nil{continue};if e=s.fetchSource(ctx,src);e!=nil{log.Printf("rss %s: %v",src.Name,e)}}}
func (s *server) fetchSource(ctx context.Context,src source)error{feed,e:=s.feed.ParseURLWithContext(src.FeedURL,ctx);if e!=nil{return e};for _,item:=range feed.Items{link:=normalizeURL(item.Link);title:=strings.TrimSpace(item.Title);if link==""||title==""{continue};var exists int;e=s.db.QueryRowContext(ctx,"SELECT 1 FROM articles WHERE url=?",link).Scan(&exists);if e==nil{continue};if !errors.Is(e,sql.ErrNoRows){return e};published:=time.Now().UTC();if item.PublishedParsed!=nil{published=*item.PublishedParsed}else if item.UpdatedParsed!=nil{published=*item.UpdatedParsed};image:="";if item.Image!=nil{image=item.Image.URL};if image==""&&len(item.Enclosures)>0{image=item.Enclosures[0].URL};desc:=strings.TrimSpace(item.Description);if desc==""{desc=strings.TrimSpace(item.Content)};summary:=s.summarize(ctx,title,desc);_,e=s.db.ExecContext(ctx,`INSERT INTO articles(source_id,category_id,title,description,summary,url,image_url,published_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(url) DO NOTHING`,src.ID,src.CategoryID,title,desc,summary,link,image,published.UTC().Format(time.RFC3339));if e!=nil{return e}};return nil}
func normalizeURL(raw string) string { u,e:=url.Parse(strings.TrimSpace(raw)); if e!=nil || u.Scheme=="" || u.Host=="" { return "" }; u.Fragment=""; u.Scheme=strings.ToLower(u.Scheme); u.Host=strings.ToLower(u.Host); return u.String() }

func (s *server) summarize(ctx context.Context,title,body string) string { fallback:=strings.TrimSpace(body);if s.cfg.AIURL==""||s.cfg.AIKey==""{return fallback};text:=strings.TrimSpace(title+"\n\n"+body);if len(text)>6000{text=text[:6000]};payload:=map[string]any{"model":s.cfg.AIModel,"messages":[]map[string]string{{"role":"system","content":"You summarize news for a Vietnamese reader. Return only a concise Vietnamese summary in at most two sentences. Translate essential information faithfully; do not add facts."},{"role":"user","content":text}},"temperature":0.2};data,e:=json.Marshal(payload);if e!=nil{return fallback};req,e:=http.NewRequestWithContext(ctx,http.MethodPost,s.cfg.AIURL,strings.NewReader(string(data)));if e!=nil{return fallback};req.Header.Set("Authorization","Bearer "+s.cfg.AIKey);req.Header.Set("Content-Type","application/json");res,e:=(&http.Client{Timeout:30*time.Second}).Do(req);if e!=nil{log.Printf("AI summary: %v",e);return fallback};defer res.Body.Close();if res.StatusCode<200||res.StatusCode>=300{log.Printf("AI summary: endpoint returned %s",res.Status);return fallback};var out struct{Choices []struct{Message struct{Content string `json:"content"`} `json:"message"`} `json:"choices"`};if e=json.NewDecoder(res.Body).Decode(&out);e!=nil||len(out.Choices)==0{if e!=nil{log.Printf("AI summary decode: %v",e)};return fallback};summary:=strings.TrimSpace(out.Choices[0].Message.Content);if summary==""{return fallback};return summary}

func (s *server) runBot(ctx context.Context){offset:=0;client:=&http.Client{Timeout:40*time.Second};for ctx.Err()==nil{var out struct{OK bool `json:"ok"`;Result []struct{UpdateID int `json:"update_id"`;Message *struct{Chat struct{ID int64 `json:"id"`} `json:"chat"`;Text string `json:"text"`} `json:"message"`} `json:"result"`};u:=fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=25&offset=%d",s.cfg.BotToken,offset);req,e:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil);if e!=nil{return};res,e:=client.Do(req);if e!=nil{if ctx.Err()==nil{log.Printf("bot poll: %v",e);time.Sleep(2*time.Second)};continue};body,_:=io.ReadAll(res.Body);res.Body.Close();if e=json.Unmarshal(body,&out);e!=nil||!out.OK{log.Printf("bot response: %v",e);time.Sleep(2*time.Second);continue};for _,up:=range out.Result{offset=up.UpdateID+1;if up.Message!=nil&&strings.HasPrefix(up.Message.Text,"/start"){s.sendWebApp(ctx,client,up.Message.Chat.ID)}}}}
func (s *server) sendWebApp(ctx context.Context,c *http.Client,chatID int64){payload:=map[string]any{"chat_id":chatID,"text":"Open your personal news feed:","reply_markup":map[string]any{"inline_keyboard":[][]any{{map[string]any{"text":"Open News","web_app":map[string]string{"url":s.cfg.MiniAppURL}}}}}};b,_:=json.Marshal(payload);req,e:=http.NewRequestWithContext(ctx,http.MethodPost,"https://api.telegram.org/bot"+s.cfg.BotToken+"/sendMessage",strings.NewReader(string(b)));if e!=nil{return};req.Header.Set("Content-Type","application/json");if _,e=c.Do(req);e!=nil{log.Printf("bot send: %v",e)}}
