package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/mmcdole/gofeed"
)

func TestMigrateSeedsVietnamNewsFeeds(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	// A second migration must retain one row per feed URL.
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}

	want := map[string]struct {
		name, category string
	}{
		"https://vietnamnews.vn/rss/travel.rss":                    {"Vietnam News – Travel", "culture"},
		"https://vietnamnews.vn/rss/a-twist-in-the-tale.rss":       {"Vietnam News – A Twist in the Tale", "culture"},
		"https://vietnamnews.vn/rss/english-through-the-news.rss":  {"Vietnam News – English Through the News", "education"},
		"https://vietnamnews.vn/rss/bizhub.rss":                    {"Vietnam News – Bizhub", "business"},
		"https://vietnamnews.vn/rss/ovietnam.rss":                  {"Vietnam News – OVietnam", "culture"},
		"https://vietnamnews.vn/rss/politics-laws.rss":             {"Vietnam News – Politics & Laws", "world"},
		"https://vietnamnews.vn/rss/society.rss":                   {"Vietnam News – Society", "society"},
		"https://vietnamnews.vn/rss/talk-around-town.rss":          {"Vietnam News – Talk Around Town", "society"},
		"https://vietnamnews.vn/rss/economy.rss":                   {"Vietnam News – Economy", "business"},
		"https://vietnamnews.vn/rss/life-style.rss":                {"Vietnam News – Life & Style", "culture"},
		"https://vietnamnews.vn/rss/sports.rss":                    {"Vietnam News – Sports", "sports"},
		"https://vietnamnews.vn/rss/environment.rss":               {"Vietnam News – Environment", "science"},
		"https://vietnamnews.vn/rss/domestic-press-highlights.rss": {"Vietnam News – Domestic Press Highlights", "world"},
		"https://vietnamnews.vn/rss/opinion.rss":                   {"Vietnam News – Opinion", "world"},
		"https://vietnamnews.vn/rss/industries.rss":                {"Vietnam News – Industries", "business"},
		"https://vietnamnews.vn/rss/agriculture.rss":               {"Vietnam News – Agriculture", "business"},
		"https://vietnamnews.vn/rss/world.rss":                     {"Vietnam News – World", "world"},
		"https://vietnamnews.vn/rss/sunday.rss":                    {"Vietnam News – Sunday", "culture"},
	}

	for url, source := range want {
		var name, category, countryCode, countryName string
		err = db.QueryRow(`SELECT s.name, c.slug, s.country_code, s.country_name
			FROM sources s JOIN categories c ON c.id = s.category_id WHERE s.feed_url = ?`, url).
			Scan(&name, &category, &countryCode, &countryName)
		if err != nil {
			t.Fatalf("seeded source %q: %v", url, err)
		}
		if name != source.name || category != source.category || countryCode != "VN" || countryName != "Việt Nam" {
			t.Errorf("source %q = (%q, %q, %q, %q), want (%q, %q, %q, %q)", url, name, category, countryCode, countryName, source.name, source.category, "VN", "Việt Nam")
		}
	}

	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM sources WHERE feed_url LIKE 'https://vietnamnews.vn/rss/%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(want) {
		t.Fatalf("Vietnam News feed count = %d, want %d", count, len(want))
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM sources WHERE feed_url = 'https://vietnamnews.vn/rss/brandinfo.rss'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("Brandinfo feed count = %d, want 0", count)
	}
}

func TestDecodeUTF16Feed(t *testing.T) {
	const rss = `<?xml version="1.0" encoding="utf-16"?><rss version="2.0"><channel><title>Vietnam News</title><item><title>Sample</title><link>https://example.test/sample</link></item></channel></rss>`
	encoded := utf16.Encode([]rune(rss))
	body := make([]byte, 0, len(encoded)*2)
	for _, unit := range encoded {
		body = append(body, byte(unit), byte(unit>>8))
	}

	decoded := decodeUTF16Feed(body)
	want := strings.Replace(rss, "utf-16", "utf-8", 1)
	if !bytes.Equal(decoded, []byte(want)) {
		t.Fatalf("decoded UTF-16 feed = %q, want %q", decoded, want)
	}
	feed, err := gofeed.NewParser().Parse(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("parse decoded feed: %v", err)
	}
	if len(feed.Items) != 1 || feed.Items[0].Title != "Sample" {
		t.Fatalf("parsed items = %#v, want one Sample item", feed.Items)
	}
}

func TestMigrateSeedsVietnamPlusFeeds(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"https://www.vietnamplus.vn/rss/chinhtri-291.rss":                      "world",
		"https://www.vietnamplus.vn/rss/thegioi-209.rss":                       "world",
		"https://www.vietnamplus.vn/rss/thegioi/asean-356.rss":                 "world",
		"https://www.vietnamplus.vn/rss/thegioi/chaua-tbd-352.rss":             "world",
		"https://www.vietnamplus.vn/rss/kinhte-311.rss":                        "business",
		"https://www.vietnamplus.vn/rss/kinhte/kinhdoanh-342.rss":              "business",
		"https://www.vietnamplus.vn/rss/kinhte/taichinh-343.rss":               "business",
		"https://www.vietnamplus.vn/rss/kinhte/tindung-385.rss":                "business",
		"https://www.vietnamplus.vn/rss/kinhte/chungkhoan-344.rss":             "business",
		"https://www.vietnamplus.vn/rss/kinhte/batdongsan-372.rss":             "business",
		"https://www.vietnamplus.vn/rss/kinhte/doanhnghiep-345.rss":            "business",
		"https://www.vietnamplus.vn/rss/kinhte/thong-tin-doanh-nghiep-433.rss": "business",
		"https://www.vietnamplus.vn/rss/kinhte/thong-cao-bao-chi-426.rss":      "business",
		"https://www.vietnamplus.vn/rss/xahoi-314.rss":                         "society",
		"https://www.vietnamplus.vn/rss/xahoi/giaoduc-316.rss":                 "education",
		"https://www.vietnamplus.vn/rss/xahoi/yte-325.rss":                     "health",
		"https://www.vietnamplus.vn/rss/xahoi/phapluat-327.rss":                "society",
		"https://www.vietnamplus.vn/rss/khoahoc-213.rss":                       "science",
		"https://www.vietnamplus.vn/rss/khoahoc/ungdung-371.rss":               "science",
		"https://www.vietnamplus.vn/rss/congnghe-212.rss":                      "technology",
		"https://www.vietnamplus.vn/rss/congnghe/sanphammoi-275.rss":           "technology",
		"https://www.vietnamplus.vn/rss/otoxemay-364.rss":                      "technology",
		"https://www.vietnamplus.vn/rss/moitruong-270.rss":                     "science",
		"https://www.vietnamplus.vn/rss/tinthitruong-218.rss":                  "business",
		"https://www.vietnamplus.vn/rss/chuyenla-329.rss":                      "culture",
		"https://www.vietnamplus.vn/rss/special-plus-450.rss":                  "culture",
		"https://www.vietnamplus.vn/rss/xahoi/giaothong-358.rss":               "society",
		"https://www.vietnamplus.vn/rss/xahoi/vietkieu-373.rss":                "world",
		"https://www.vietnamplus.vn/rss/vanhoa-215.rss":                        "culture",
	}
	for url, category := range want {
		var name, gotCategory, countryCode, countryName string
		err = db.QueryRow(`SELECT s.name, c.slug, s.country_code, s.country_name
			FROM sources s JOIN categories c ON c.id = s.category_id WHERE s.feed_url = ?`, url).
			Scan(&name, &gotCategory, &countryCode, &countryName)
		if err != nil {
			t.Fatalf("seeded source %q: %v", url, err)
		}
		if !strings.HasPrefix(name, "VietnamPlus – ") || gotCategory != category || countryCode != "VN" || countryName != "Việt Nam" {
			t.Errorf("source %q = (%q, %q, %q, %q), want VietnamPlus name, (%q, %q, %q)", url, name, gotCategory, countryCode, countryName, category, "VN", "Việt Nam")
		}
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM sources WHERE feed_url LIKE 'https://www.vietnamplus.vn/rss/%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(want) {
		t.Fatalf("VietnamPlus feed count = %d, want %d", count, len(want))
	}
}
