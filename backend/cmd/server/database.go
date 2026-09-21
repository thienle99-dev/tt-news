package main

import (
	"database/sql"
	"strings"

	"telegram-news/internal/crawlers/rss"
	"telegram-news/internal/crawlers/scmp"
)

func health(dbPath string) bool {
	db, err := openDB(dbPath)
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}

func openDB(file string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	for _, query := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err = db.Exec(query); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	var err error
	for _, name := range []string{"migrations/001_init.sql", "migrations/002_translations.sql", "migrations/003_article_content_images.sql", "migrations/004_translation_jobs.sql", "migrations/005_remove_reuters.sql", "migrations/007_featured_briefs.sql", "migrations/008_reading_history.sql", "migrations/009_saved_organization.sql", "migrations/010_ai_feedback.sql", "migrations/011_source_health.sql", "migrations/012_ai_config.sql", "migrations/013_article_categories.sql", "migrations/014_daily_brief.sql", "migrations/015_medium_reader.sql", "migrations/016_job_runs.sql", "migrations/017_job_progress.sql", "migrations/018_ai_usage.sql", "migrations/019_article_content_fetches.sql", "migrations/020_article_content_extractor_version.sql", "migrations/021_daily_digest.sql", "migrations/022_article_watches.sql", "migrations/023_saved_filters.sql", "migrations/024_threads.sql", "migrations/025_threads_discovery.sql", "migrations/026_threads_author_blocks.sql", "migrations/027_threads_comments.sql"} {
		var schema []byte
		schema, err = embedded.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err = db.Exec(string(schema)); err != nil && !(name == "migrations/003_article_content_images.sql" && strings.Contains(err.Error(), "duplicate column name")) && !(name == "migrations/017_job_progress.sql" && strings.Contains(err.Error(), "duplicate column name")) && !(name == "migrations/020_article_content_extractor_version.sql" && strings.Contains(err.Error(), "duplicate column name")) && !(name == "migrations/025_threads_discovery.sql" && strings.Contains(err.Error(), "duplicate column name")) {
			return err
		}
	}

	if _, err = db.Exec("ALTER TABLE articles ADD COLUMN summary TEXT NOT NULL DEFAULT ''"); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	for _, statement := range []string{
		"ALTER TABLE sources ADD COLUMN country_code TEXT NOT NULL DEFAULT 'GLOBAL'",
		"ALTER TABLE sources ADD COLUMN country_name TEXT NOT NULL DEFAULT 'Toàn cầu'",
		"ALTER TABLE articles ADD COLUMN title_fingerprint TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN content_fingerprint TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN full_content TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN content_reviewed_at TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN content_review_version TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE featured_topics ADD COLUMN why_it_matters TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE featured_topic_translations ADD COLUMN why_it_matters TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN thread_post_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN thread_author TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN thread_display_name TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN thread_avatar_url TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE articles ADD COLUMN thread_likes INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE articles ADD COLUMN thread_replies INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE articles ADD COLUMN thread_reposts INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE articles ADD COLUMN is_hidden INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE articles ADD COLUMN hidden_at TEXT NOT NULL DEFAULT ''",
	} {
		if _, err = db.Exec(statement); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	// Logged-out Threads pages may expose JavaScript hydration data instead of
	// a post caption. Run after compatibility columns have been added so this
	// also works for databases created before Threads support existed.
	if _, err = db.Exec(`DELETE FROM articles WHERE thread_post_id<>'' AND (
		lower(description) LIKE '%bootstrapwebsession%' OR lower(description) LIKE '%cometssr%' OR
		lower(description) LIKE '%qpltagserverjs%' OR lower(description) LIKE '%qpltimingsserverjs%' OR
		lower(description) LIKE '%replacenativetimer%' OR lower(description) LIKE '%maybedisableanimations%')`); err != nil {
		return err
	}
	// Fingerprints are filled for newly imported articles. Partial indexes let
	// older rows remain untouched while protecting future imports.
	fingerprintIndexes, err := embedded.ReadFile("migrations/006_article_fingerprints.sql")
	if err != nil {
		return err
	}
	if _, err = db.Exec(string(fingerprintIndexes)); err != nil {
		return err
	}

	categories := []struct{ slug, name string }{
		{"technology", "Công nghệ"},
		{"programming", "Lập trình"},
		{"world", "Thế giới"},
		{"business", "Kinh doanh"},
		{"society", "Xã hội"},
		{"culture", "Văn hóa"},
		{"sports", "Thể thao"},
		{"education", "Giáo dục"},
		{"health", "Sức khỏe"},
		{"science", "Khoa học"},
	}
	for _, category := range rssItemCategoryDefinitions {
		categories = append(categories, struct{ slug, name string }{category.slug, category.name})
	}
	for _, category := range categories {
		if _, err = db.Exec("INSERT INTO categories(slug,name) VALUES(?,?) ON CONFLICT(slug) DO UPDATE SET name=excluded.name", category.slug, category.name); err != nil {
			return err
		}
	}

	sources := []rss.Source{
		{Name: "Hacker News", URL: "https://news.ycombinator.com/rss", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "DEV Community", URL: "https://dev.to/feed", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "daily.dev", URL: "https://daily.dev/rss.xml", Category: "programming", CountryCode: "IL", CountryName: "Israel"},
		{Name: "GitHub Blog", URL: "https://github.blog/feed/", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Go Blog", URL: "https://go.dev/blog/feed.atom", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Golang Weekly", URL: "https://golangweekly.com/rss", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Simon Willison", URL: "https://simonwillison.net/atom/everything/", Category: "programming", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "ByteByteGo", URL: "https://blog.bytebytego.com/feed", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Martin Fowler", URL: "https://martinfowler.com/feed.atom", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Stack Overflow Blog", URL: "https://stackoverflow.blog/feed/", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "CSS-Tricks", URL: "https://css-tricks.com/feed/", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Meta Engineering", URL: "https://engineering.fb.com/feed/", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Netflix TechBlog", URL: "https://netflixtechblog.com/feed", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Slack Engineering", URL: "https://slack.engineering/feed/", Category: "programming", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Ars Technica", URL: "https://feeds.arstechnica.com/arstechnica/index", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Krebs on Security", URL: "https://krebsonsecurity.com/feed/", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Discord Blog", URL: "https://discord.com/blog/rss.xml", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Latent Space", URL: "https://www.latent.space/feed", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Goon's Solo Playbook", URL: "https://goonnguyen.substack.com/feed", Category: "technology", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "AutoDaily", URL: "https://forum.autodaily.vn/forums/tin-tuc/index.rss", Category: "technology", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Product Hunt", URL: "https://www.producthunt.com/feed", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "Al Jazeera English", URL: "https://www.aljazeera.com/xml/rss/all.xml", Category: "world", CountryCode: "QA", CountryName: "Qatar"},
		// {Name: "The Guardian World", URL: "https://www.theguardian.com/world/rss", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "The Guardian Politics", URL: "https://www.theguardian.com/politics/rss", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "The Guardian China", URL: "https://www.theguardian.com/world/china/rss", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "Euronews World", URL: "https://www.euronews.com/rss?format=mrss&level=theme&name=news", Category: "world", CountryCode: "EU", CountryName: "Liên minh châu Âu"},
		// {Name: "Euronews My Europe", URL: "https://www.euronews.com/rss?format=mrss&level=vertical&name=my-europe", Category: "world", CountryCode: "EU", CountryName: "Liên minh châu Âu"},
		// {Name: "VOA Africa", URL: "https://www.voanews.com/api/z-botl-vomx-tpertmq", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "VOA East Asia", URL: "https://www.voanews.com/api/zobo_l-vomx-tpepvmv", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "VOA China", URL: "https://www.voanews.com/api/zmjuqtl-vomx-tpey_jqq", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "VOA South & Central Asia", URL: "https://www.voanews.com/api/z_-mqyl-vomx-tpevyvqv", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "VOA Middle East", URL: "https://www.voanews.com/api/zrbopl-vomx-tpeovm_", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "VOA Europe", URL: "https://www.voanews.com/api/zjbovl-vomx-tpebvmr", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "VOA Ukraine", URL: "https://www.voanews.com/api/zt_rqyl-vomx-tpekboq_", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "VOA Americas", URL: "https://www.voanews.com/api/zoripl-vomx-tpeptmm", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "DW News", URL: "https://rss.dw.com/rdf/rss-en-all", Category: "world", CountryCode: "DE", CountryName: "Đức"},
		// {Name: "CNA World", URL: "https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml&category=6311", Category: "world", CountryCode: "SG", CountryName: "Singapore"},
		// {Name: "CNA Asia", URL: "https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml&category=6511", Category: "world", CountryCode: "SG", CountryName: "Singapore"},
		// {Name: "POLITICO Politics", URL: "https://rss.politico.com/politics-news.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "POLITICO Congress", URL: "https://rss.politico.com/congress.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "POLITICO Playbook", URL: "https://rss.politico.com/playbook.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "POLITICO Magazine", URL: "https://rss.politico.com/magazine.xml", Category: "culture", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "POLITICO Economy", URL: "https://rss.politico.com/economy.xml", Category: "business", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "POLITICO Energy", URL: "https://rss.politico.com/energy.xml", Category: "business", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "BBC World", URL: "https://feeds.bbci.co.uk/news/world/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC News Tiếng Việt", URL: "https://feeds.bbci.co.uk/vietnamese/rss.xml", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VOA Tiếng Việt", URL: "https://www.voatiengviet.com/api/zbuyil-vomx-tpeqpum", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "RFA Tiếng Việt", URL: "https://www.rfa.org/arc/outboundfeeds/vietnamese/rss/", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "RFI Tiếng Việt", URL: "https://www.rfi.fr/vi/rss", Category: "world", CountryCode: "FR", CountryName: "Pháp"},
		{Name: "KBS World Tiếng Việt", URL: "https://world.kbs.co.kr/rss/rss_news.htm?lang=v", Category: "world", CountryCode: "KR", CountryName: "Hàn Quốc"},
		{Name: "The Verge", URL: "https://www.theverge.com/rss/index.xml", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "TechCrunch", URL: "https://techcrunch.com/feed/", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Engadget", URL: "https://www.engadget.com/rss.xml", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "404 Media", URL: "https://www.404media.co/rss/", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "9to5Mac", URL: "https://9to5mac.com/feed/", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Macworld", URL: "https://www.macworld.com/feed", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Android Authority", URL: "https://www.androidauthority.com/feed/", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "TechNode", URL: "https://technode.com/feed/", Category: "technology", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Gizmochina", URL: "https://www.gizmochina.com/feed/", Category: "technology", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Wccftech", URL: "https://wccftech.com/feed/", Category: "technology", CountryCode: "CA", CountryName: "Canada"},
		{Name: "BGR", URL: "https://bgr.com/feed/", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "9to5Google", URL: "https://9to5google.com/feed/", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "CafeF – Tổng hợp", URL: "https://cafef.vn/home.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Chứng khoán", URL: "https://cafef.vn/thi-truong-chung-khoan.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Doanh nghiệp", URL: "https://cafef.vn/doanh-nghiep.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Tài chính - Ngân hàng", URL: "https://cafef.vn/tai-chinh-ngan-hang.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Bất động sản", URL: "https://cafef.vn/bat-dong-san.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Thị trường", URL: "https://cafef.vn/thi-truong.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Vĩ mô - Đầu tư", URL: "https://cafef.vn/vi-mo-dau-tu.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Tài chính quốc tế", URL: "https://cafef.vn/tai-chinh-quoc-te.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Smart Money", URL: "https://cafef.vn/smart-money.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Kinh tế số", URL: "https://cafef.vn/kinh-te-so.rss", Category: "technology", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "CafeF – Lifestyle", URL: "https://cafef.vn/lifestyle.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Travel", URL: "https://vietnamnews.vn/rss/travel.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – A Twist in the Tale", URL: "https://vietnamnews.vn/rss/a-twist-in-the-tale.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – English Through the News", URL: "https://vietnamnews.vn/rss/english-through-the-news.rss", Category: "education", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Bizhub", URL: "https://vietnamnews.vn/rss/bizhub.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – OVietnam", URL: "https://vietnamnews.vn/rss/ovietnam.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Politics & Laws", URL: "https://vietnamnews.vn/rss/politics-laws.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Society", URL: "https://vietnamnews.vn/rss/society.rss", Category: "society", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Talk Around Town", URL: "https://vietnamnews.vn/rss/talk-around-town.rss", Category: "society", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Economy", URL: "https://vietnamnews.vn/rss/economy.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Life & Style", URL: "https://vietnamnews.vn/rss/life-style.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Sports", URL: "https://vietnamnews.vn/rss/sports.rss", Category: "sports", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Environment", URL: "https://vietnamnews.vn/rss/environment.rss", Category: "science", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Domestic Press Highlights", URL: "https://vietnamnews.vn/rss/domestic-press-highlights.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Opinion", URL: "https://vietnamnews.vn/rss/opinion.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Industries", URL: "https://vietnamnews.vn/rss/industries.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Agriculture", URL: "https://vietnamnews.vn/rss/agriculture.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – World", URL: "https://vietnamnews.vn/rss/world.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "Vietnam News – Sunday", URL: "https://vietnamnews.vn/rss/sunday.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Chính trị", URL: "https://www.vietnamplus.vn/rss/chinhtri-291.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Thế giới", URL: "https://www.vietnamplus.vn/rss/thegioi-209.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – ASEAN", URL: "https://www.vietnamplus.vn/rss/thegioi/asean-356.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Châu Á - TBD", URL: "https://www.vietnamplus.vn/rss/thegioi/chaua-tbd-352.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Kinh tế", URL: "https://www.vietnamplus.vn/rss/kinhte-311.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Kinh doanh", URL: "https://www.vietnamplus.vn/rss/kinhte/kinhdoanh-342.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Tài chính", URL: "https://www.vietnamplus.vn/rss/kinhte/taichinh-343.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Tín dụng nông thôn", URL: "https://www.vietnamplus.vn/rss/kinhte/tindung-385.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Chứng khoán", URL: "https://www.vietnamplus.vn/rss/kinhte/chungkhoan-344.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Bất động sản", URL: "https://www.vietnamplus.vn/rss/kinhte/batdongsan-372.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Doanh nghiệp", URL: "https://www.vietnamplus.vn/rss/kinhte/doanhnghiep-345.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Thông tin doanh nghiệp", URL: "https://www.vietnamplus.vn/rss/kinhte/thong-tin-doanh-nghiep-433.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Thông cáo báo chí", URL: "https://www.vietnamplus.vn/rss/kinhte/thong-cao-bao-chi-426.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Xã hội", URL: "https://www.vietnamplus.vn/rss/xahoi-314.rss", Category: "society", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Giáo dục", URL: "https://www.vietnamplus.vn/rss/xahoi/giaoduc-316.rss", Category: "education", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Y tế", URL: "https://www.vietnamplus.vn/rss/xahoi/yte-325.rss", Category: "health", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Pháp luật", URL: "https://www.vietnamplus.vn/rss/xahoi/phapluat-327.rss", Category: "society", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Khoa học", URL: "https://www.vietnamplus.vn/rss/khoahoc-213.rss", Category: "science", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Khoa học ứng dụng", URL: "https://www.vietnamplus.vn/rss/khoahoc/ungdung-371.rss", Category: "science", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Công nghệ", URL: "https://www.vietnamplus.vn/rss/congnghe-212.rss", Category: "technology", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Sản phẩm mới", URL: "https://www.vietnamplus.vn/rss/congnghe/sanphammoi-275.rss", Category: "technology", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Ôtô - Xe máy", URL: "https://www.vietnamplus.vn/rss/otoxemay-364.rss", Category: "technology", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Môi trường", URL: "https://www.vietnamplus.vn/rss/moitruong-270.rss", Category: "science", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Thị trường", URL: "https://www.vietnamplus.vn/rss/tinthitruong-218.rss", Category: "business", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Chuyện lạ", URL: "https://www.vietnamplus.vn/rss/chuyenla-329.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Special+", URL: "https://www.vietnamplus.vn/rss/special-plus-450.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Giao thông", URL: "https://www.vietnamplus.vn/rss/xahoi/giaothong-358.rss", Category: "society", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Người Việt bốn phương", URL: "https://www.vietnamplus.vn/rss/xahoi/vietkieu-373.rss", Category: "world", CountryCode: "VN", CountryName: "Việt Nam"},
		{Name: "VietnamPlus – Văn hóa", URL: "https://www.vietnamplus.vn/rss/vanhoa-215.rss", Category: "culture", CountryCode: "VN", CountryName: "Việt Nam"},
		// {Name: "BBC Business", URL: "https://feeds.bbci.co.uk/news/business/rss.xml", Category: "business", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC News", URL: "https://feeds.bbci.co.uk/news/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC UK", URL: "https://feeds.bbci.co.uk/news/uk/rss.xml", Category: "society", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Technology", URL: "https://feeds.bbci.co.uk/news/technology/rss.xml", Category: "technology", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Science & Environment", URL: "https://feeds.bbci.co.uk/news/science_and_environment/rss.xml", Category: "science", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Entertainment & Arts", URL: "https://feeds.bbci.co.uk/news/entertainment_and_arts/rss.xml", Category: "culture", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Asia", URL: "https://feeds.bbci.co.uk/news/world/asia/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Europe", URL: "https://feeds.bbci.co.uk/news/world/europe/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Africa", URL: "https://feeds.bbci.co.uk/news/world/africa/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Middle East", URL: "https://feeds.bbci.co.uk/news/world/middle_east/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC US & Canada", URL: "https://feeds.bbci.co.uk/news/world/us_and_canada/rss.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		// {Name: "BBC Latin America", URL: "https://feeds.bbci.co.uk/news/world/latin_america/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Politics", URL: "https://feeds.bbci.co.uk/news/politics/rss.xml", Category: "society", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "BBC Sport", URL: "https://feeds.bbci.co.uk/sport/rss.xml", Category: "sports", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		// {Name: "People's Daily – Latest", URL: "http://www.people.com.cn/rss/ywkx.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "People's Daily – Politics", URL: "http://www.people.com.cn/rss/politics.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "People's Daily – World", URL: "http://www.people.com.cn/rss/world.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "People's Daily – Society", URL: "http://www.people.com.cn/rss/society.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "People's Daily – Military", URL: "http://www.people.com.cn/rss/military.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Politics", URL: "http://www.xinhuanet.com/politics/news_politics.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – World", URL: "http://www.xinhuanet.com/world/news_world.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Local", URL: "http://www.xinhuanet.com/local/news_province.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Military", URL: "http://www.xinhuanet.com/mil/news_mil.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Taiwan", URL: "http://www.xinhuanet.com/tw/news_tw.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Overseas Chinese", URL: "http://www.xinhuanet.com/overseas/news_overseas.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Finance", URL: "http://www.xinhuanet.com/finance/news_finance.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Economy", URL: "http://www.xinhuanet.com/fortune/news_fortune.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "Xinhua – Property", URL: "http://www.xinhuanet.com/house/news_house.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Latest", URL: "https://www.chinanews.com.cn/rss/scroll-news.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Highlights", URL: "https://www.chinanews.com.cn/rss/importnews.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – China", URL: "https://www.chinanews.com.cn/rss/china.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – World", URL: "https://www.chinanews.com.cn/rss/world.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Society", URL: "https://www.chinanews.com.cn/rss/society.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Finance", URL: "https://www.chinanews.com.cn/rss/finance.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Overseas Chinese", URL: "https://www.chinanews.com.cn/rss/chinese.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Culture", URL: "https://www.chinanews.com.cn/rss/culture.xml", Category: "culture", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Sports", URL: "https://www.chinanews.com.cn/rss/sports.xml", Category: "sports", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Education", URL: "https://www.chinanews.com.cn/rss/edu.xml", Category: "education", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Health", URL: "https://www.chinanews.com.cn/rss/jk.xml", Category: "health", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Video", URL: "https://www.chinanews.com.cn/rss/sp.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		// {Name: "China News Service – Photo", URL: "https://www.chinanews.com.cn/rss/photo.xml", Category: "culture", CountryCode: "CN", CountryName: "Trung Quốc"},
	}
	sources = append(sources, scmp.Feeds...)
	sources = append(sources, rss.Source{Name: "Threads", URL: "https://www.threads.com", Category: "technology", CountryCode: "GLOBAL", CountryName: "Toàn cầu"})
	if _, err = db.Exec(`UPDATE sources SET feed_url=? WHERE name=? AND feed_url=?`, "https://news.ycombinator.com/rss", "Hacker News", "https://hnrss.org/frontpage"); err != nil {
		return err
	}
	if _, err = db.Exec(`UPDATE sources SET feed_url=? WHERE name=? AND feed_url=?`, "https://world.kbs.co.kr/rss/rss_news.htm?lang=v", "KBS World Tiếng Việt", "https://world.kbs.co.kr/rss/rss_vi.xml"); err != nil {
		return err
	}
	for _, source := range sources {
		if source.CountryCode == "" {
			source.CountryCode, source.CountryName = "GLOBAL", "Toàn cầu"
		}
		if _, err = db.Exec(`INSERT INTO sources(name,feed_url,category_id,country_code,country_name) VALUES(?,?,(SELECT id FROM categories WHERE slug=?),?,?) ON CONFLICT(feed_url) DO UPDATE SET country_code=excluded.country_code,country_name=excluded.country_name`, source.Name, source.URL, source.Category, source.CountryCode, source.CountryName); err != nil {
			return err
		}
	}
	return nil
}
