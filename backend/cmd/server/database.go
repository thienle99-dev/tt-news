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
	for _, name := range []string{"migrations/001_init.sql", "migrations/002_translations.sql", "migrations/003_article_content_images.sql", "migrations/004_translation_jobs.sql", "migrations/005_remove_reuters.sql"} {
		var schema []byte
		schema, err = embedded.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err = db.Exec(string(schema)); err != nil && !(name == "migrations/003_article_content_images.sql" && strings.Contains(err.Error(), "duplicate column name")) {
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
	} {
		if _, err = db.Exec(statement); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
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
		{"world", "Thế giới"},
		{"business", "Kinh doanh"},
		{"society", "Xã hội"},
		{"culture", "Văn hóa"},
		{"sports", "Thể thao"},
		{"education", "Giáo dục"},
		{"health", "Sức khỏe"},
		{"science", "Khoa học"},
	}
	for _, category := range categories {
		if _, err = db.Exec("INSERT INTO categories(slug,name) VALUES(?,?) ON CONFLICT(slug) DO UPDATE SET name=excluded.name", category.slug, category.name); err != nil {
			return err
		}
	}

	sources := []rss.Source{
		{Name: "Hacker News", URL: "https://hnrss.org/frontpage", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "Al Jazeera English", URL: "https://www.aljazeera.com/xml/rss/all.xml", Category: "world", CountryCode: "QA", CountryName: "Qatar"},
		{Name: "The Guardian World", URL: "https://www.theguardian.com/world/rss", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "The Guardian Politics", URL: "https://www.theguardian.com/politics/rss", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "The Guardian China", URL: "https://www.theguardian.com/world/china/rss", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "Euronews World", URL: "https://www.euronews.com/rss?format=mrss&level=theme&name=news", Category: "world", CountryCode: "EU", CountryName: "Liên minh châu Âu"},
		{Name: "Euronews My Europe", URL: "https://www.euronews.com/rss?format=mrss&level=vertical&name=my-europe", Category: "world", CountryCode: "EU", CountryName: "Liên minh châu Âu"},
		{Name: "VOA Africa", URL: "https://www.voanews.com/api/z-botl-vomx-tpertmq", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "VOA East Asia", URL: "https://www.voanews.com/api/zobo_l-vomx-tpepvmv", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "VOA China", URL: "https://www.voanews.com/api/zmjuqtl-vomx-tpey_jqq", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "VOA South & Central Asia", URL: "https://www.voanews.com/api/z_-mqyl-vomx-tpevyvqv", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "VOA Middle East", URL: "https://www.voanews.com/api/zrbopl-vomx-tpeovm_", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "VOA Europe", URL: "https://www.voanews.com/api/zjbovl-vomx-tpebvmr", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "VOA Ukraine", URL: "https://www.voanews.com/api/zt_rqyl-vomx-tpekboq_", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "VOA Americas", URL: "https://www.voanews.com/api/zoripl-vomx-tpeptmm", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "DW News", URL: "https://rss.dw.com/rdf/rss-en-all", Category: "world", CountryCode: "DE", CountryName: "Đức"},
		{Name: "CNA World", URL: "https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml&category=6311", Category: "world", CountryCode: "SG", CountryName: "Singapore"},
		{Name: "CNA Asia", URL: "https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml&category=6511", Category: "world", CountryCode: "SG", CountryName: "Singapore"},
		{Name: "POLITICO Politics", URL: "https://rss.politico.com/politics-news.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "POLITICO Congress", URL: "https://rss.politico.com/congress.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "POLITICO Playbook", URL: "https://rss.politico.com/playbook.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "POLITICO Magazine", URL: "https://rss.politico.com/magazine.xml", Category: "culture", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "POLITICO Economy", URL: "https://rss.politico.com/economy.xml", Category: "business", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "POLITICO Energy", URL: "https://rss.politico.com/energy.xml", Category: "business", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "BBC World", URL: "https://feeds.bbci.co.uk/news/world/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Business", URL: "https://feeds.bbci.co.uk/news/business/rss.xml", Category: "business", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC News", URL: "https://feeds.bbci.co.uk/news/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC UK", URL: "https://feeds.bbci.co.uk/news/uk/rss.xml", Category: "society", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Technology", URL: "https://feeds.bbci.co.uk/news/technology/rss.xml", Category: "technology", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Science & Environment", URL: "https://feeds.bbci.co.uk/news/science_and_environment/rss.xml", Category: "science", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Entertainment & Arts", URL: "https://feeds.bbci.co.uk/news/entertainment_and_arts/rss.xml", Category: "culture", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Asia", URL: "https://feeds.bbci.co.uk/news/world/asia/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Europe", URL: "https://feeds.bbci.co.uk/news/world/europe/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Africa", URL: "https://feeds.bbci.co.uk/news/world/africa/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Middle East", URL: "https://feeds.bbci.co.uk/news/world/middle_east/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC US & Canada", URL: "https://feeds.bbci.co.uk/news/world/us_and_canada/rss.xml", Category: "world", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "BBC Latin America", URL: "https://feeds.bbci.co.uk/news/world/latin_america/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Politics", URL: "https://feeds.bbci.co.uk/news/politics/rss.xml", Category: "society", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Sport", URL: "https://feeds.bbci.co.uk/sport/rss.xml", Category: "sports", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "People's Daily – Latest", URL: "http://www.people.com.cn/rss/ywkx.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "People's Daily – Politics", URL: "http://www.people.com.cn/rss/politics.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "People's Daily – World", URL: "http://www.people.com.cn/rss/world.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "People's Daily – Society", URL: "http://www.people.com.cn/rss/society.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "People's Daily – Military", URL: "http://www.people.com.cn/rss/military.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Politics", URL: "http://www.xinhuanet.com/politics/news_politics.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – World", URL: "http://www.xinhuanet.com/world/news_world.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Local", URL: "http://www.xinhuanet.com/local/news_province.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Military", URL: "http://www.xinhuanet.com/mil/news_mil.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Taiwan", URL: "http://www.xinhuanet.com/tw/news_tw.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Overseas Chinese", URL: "http://www.xinhuanet.com/overseas/news_overseas.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Finance", URL: "http://www.xinhuanet.com/finance/news_finance.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Economy", URL: "http://www.xinhuanet.com/fortune/news_fortune.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "Xinhua – Property", URL: "http://www.xinhuanet.com/house/news_house.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Latest", URL: "https://www.chinanews.com.cn/rss/scroll-news.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Highlights", URL: "https://www.chinanews.com.cn/rss/importnews.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – China", URL: "https://www.chinanews.com.cn/rss/china.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – World", URL: "https://www.chinanews.com.cn/rss/world.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Society", URL: "https://www.chinanews.com.cn/rss/society.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Finance", URL: "https://www.chinanews.com.cn/rss/finance.xml", Category: "business", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Overseas Chinese", URL: "https://www.chinanews.com.cn/rss/chinese.xml", Category: "world", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Culture", URL: "https://www.chinanews.com.cn/rss/culture.xml", Category: "culture", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Sports", URL: "https://www.chinanews.com.cn/rss/sports.xml", Category: "sports", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Education", URL: "https://www.chinanews.com.cn/rss/edu.xml", Category: "education", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Health", URL: "https://www.chinanews.com.cn/rss/jk.xml", Category: "health", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Video", URL: "https://www.chinanews.com.cn/rss/sp.xml", Category: "society", CountryCode: "CN", CountryName: "Trung Quốc"},
		{Name: "China News Service – Photo", URL: "https://www.chinanews.com.cn/rss/photo.xml", Category: "culture", CountryCode: "CN", CountryName: "Trung Quốc"},
	}
	sources = append(sources, scmp.Feeds...)
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
