import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { api } from "./api";
import "./featured.css";
import { FilterControls } from "./filter-controls";
import type {
  Article,
  Category,
  Country,
  FeaturedBrief,
  GoldRate,
  SavedOrganization,
  Source,
  Translation,
} from "./types";

type Tab = "home" | "featured" | "saved" | "history" | "settings";
type Locale = "en" | "vi";
type Theme = "light" | "dark";
type Filters = {
  category: string;
  source: string;
  country: string;
  query: string;
  period: string;
  sort: string;
};
const categories: Record<string, Record<Locale, string>> = {
  technology: { en: "Technology", vi: "Công nghệ" },
  programming: { en: "Programming", vi: "Lập trình" },
  world: { en: "World", vi: "Thế giới" },
  society: { en: "Society", vi: "Xã hội" },
  culture: { en: "Culture", vi: "Văn hóa" },
  sports: { en: "Sports", vi: "Thể thao" },
  education: { en: "Education", vi: "Giáo dục" },
  health: { en: "Health", vi: "Sức khỏe" },
  science: { en: "Science", vi: "Khoa học" },
  security: { en: "Security", vi: "Bảo mật" },
  ai: { en: "AI", vi: "Trí tuệ nhân tạo" },
  llm: { en: "LLM", vi: "Mô hình ngôn ngữ lớn" },
  cybersecurity: { en: "Cybersecurity", vi: "An ninh mạng" },
};
const countries: Record<string, Record<Locale, string>> = {
  GLOBAL: { en: "Global", vi: "Toàn cầu" },
  HK: { en: "Hong Kong", vi: "Hồng Kông" },
  GB: { en: "United Kingdom", vi: "Vương quốc Anh" },
  US: { en: "United States", vi: "Hoa Kỳ" },
  CN: { en: "China", vi: "Trung Quốc" },
  SG: { en: "Singapore", vi: "Singapore" },
  DE: { en: "Germany", vi: "Đức" },
  QA: { en: "Qatar", vi: "Qatar" },
  EU: { en: "European Union", vi: "Liên minh châu Âu" },
};
const text = {
  en: {
    masthead: "SIGNAL BRIEF",
    news: "Briefs",
    saved: "Saved",
    history: "Continue reading",
    clearHistory: "Clear history",
    hideRead: "Hide read",
    showRead: "Show read",
    markRead: "Mark as read",
    feedbackIncorrect: "Summary is incorrect",
    feedbackMissing: "Summary misses something",
    feedbackReason: "What is wrong or missing?",
    sendFeedback: "Send feedback",
    feedbackThanks: "Thanks — your feedback was recorded.",
    originalContent: "Review original text",
    sortSaved: "Sort saved",
    time: "Time",
    allTime: "Any time",
    last24Hours: "Last 24 hours",
    last7Days: "Last 7 days",
    sort: "Sort",
    oldest: "Oldest",
    relevance: "Most relevant",
    savedNewest: "Recently saved",
    savedOldest: "Oldest saved",
    publishedNewest: "Newest published",
    titleSort: "Title",
    folders: "Folders",
    tags: "Tags",
    addFolder: "Add folder",
    addTag: "Add tag",
    selectSaved: "Select article",
    removeSelected: "Remove selected",
    filter: "Filter",
    newest: "Latest",
    all: "All",
    featured: "FEATURED",
    featuredNews: "Featured",
    featuredUpdated: "Updated",
    featuredUnavailable: "No featured briefing is available yet.",
    yourArticles: "Your articles",
    latest: "Latest",
    articles: "briefs",
    loading: "Loading briefs…",
    loadingMore: "Loading more…",
    empty: "No matching briefs found.",
    loadError: "Could not load briefs.",
    detailError: "This article could not be loaded.",
    filters: "Filters",
    close: "Close",
    search: "Search",
    allCategories: "All categories",
    allSources: "All sources",
    allCountries: "All countries",
    viewResults: "View results",
    home: "Home",
    settings: "Settings",
    interface: "INTERFACE",
    theme: "Theme follows Telegram or your device setting.",
    switchToDark: "Switch to dark mode",
    switchToLight: "Switch to light mode",
    back: "Back",
    readOriginal: "Read original article",
    openArticle: "Open article",
    save: "Save article",
    unsave: "Remove saved article",
    translating: "Translating to Vietnamese…",
    translationError:
      "Could not translate this article. Showing the original text.",
    retry: "Try again",
    language: "Language",
    summary: "SUMMARY",
    articleContent: "ARTICLE",
    source: "Source",
    summaryEmpty: "No summary yet. Use the sparkle button to generate one.",
    goldPrices: "Gold prices",
    goldLive: "LIVE REFERENCE",
    goldBuy: "Buy",
    goldSell: "Sell",
    goldUpdated: "Updated",
    goldUnavailable: "Gold prices are temporarily unavailable.",
    goldViewSource: "View source table",
    goldDisclaimer: "Reference prices from Bảo Tín Mạnh Hải. Confirm before trading.",
  },
  vi: {
    masthead: "SIGNAL BRIEF",
    news: "Tóm tắt",
    saved: "Đã lưu",
    history: "Đọc tiếp",
    clearHistory: "Xoá lịch sử",
    hideRead: "Ẩn bài đã đọc",
    showRead: "Hiện bài đã đọc",
    markRead: "Đánh dấu đã đọc",
    feedbackIncorrect: "Tóm tắt sai",
    feedbackMissing: "Tóm tắt thiếu",
    feedbackReason: "Thông tin nào sai hoặc bị thiếu?",
    sendFeedback: "Gửi phản hồi",
    feedbackThanks: "Cảm ơn — phản hồi của bạn đã được ghi nhận.",
    originalContent: "Xem nguyên văn liên quan",
    sortSaved: "Sắp xếp bài đã lưu",
    time: "Thời gian",
    allTime: "Mọi thời điểm",
    last24Hours: "24 giờ qua",
    last7Days: "7 ngày qua",
    sort: "Sắp xếp",
    oldest: "Cũ nhất",
    relevance: "Liên quan nhất",
    savedNewest: "Mới lưu gần đây",
    savedOldest: "Lưu lâu nhất",
    publishedNewest: "Mới xuất bản",
    titleSort: "Tiêu đề",
    folders: "Thư mục",
    tags: "Nhãn",
    addFolder: "Thêm thư mục",
    addTag: "Thêm nhãn",
    selectSaved: "Chọn bài viết",
    removeSelected: "Xoá mục đã chọn",
    filter: "Lọc",
    newest: "Mới nhất",
    all: "Tất cả",
    featured: "NỔI BẬT",
    featuredNews: "Nổi bật",
    featuredUpdated: "Cập nhật",
    featuredUnavailable: "Chưa có bản tin nổi bật.",
    yourArticles: "Bài tóm tắt đã lưu",
    latest: "Mới nhất",
    articles: "bản tóm tắt",
    loading: "Đang tải bản tóm tắt…",
    loadingMore: "Đang tải thêm…",
    empty: "Không tìm thấy bản tóm tắt phù hợp.",
    loadError: "Không thể tải bản tóm tắt.",
    detailError: "Không thể tải bài viết này.",
    filters: "Bộ lọc",
    close: "Đóng",
    search: "Tìm kiếm",
    allCategories: "Tất cả thể loại",
    allSources: "Tất cả nguồn",
    allCountries: "Tất cả quốc gia",
    viewResults: "Xem kết quả",
    home: "Trang chủ",
    settings: "Cài đặt",
    interface: "GIAO DIỆN",
    theme: "Theme tự động theo Telegram hoặc thiết bị của bạn.",
    switchToDark: "Chuyển sang giao diện tối",
    switchToLight: "Chuyển sang giao diện sáng",
    back: "Quay lại",
    readOriginal: "Đọc bài gốc",
    openArticle: "Mở bài viết",
    save: "Lưu bài viết",
    unsave: "Bỏ lưu bài viết",
    translating: "Đang dịch sang tiếng Việt…",
    translationError: "Không thể dịch bài này. Đang hiển thị nội dung gốc.",
    retry: "Thử lại",
    language: "Ngôn ngữ",
    summary: "TÓM TẮT",
    articleContent: "NỘI DUNG BÀI VIẾT",
    source: "Nguồn",
    summaryEmpty: "Chưa có bản tóm tắt. Nhấn nút ở góc phải để AI tạo tóm tắt.",
    goldPrices: "Giá vàng",
    goldLive: "THAM KHẢO TRỰC TIẾP",
    goldBuy: "Mua",
    goldSell: "Bán",
    goldUpdated: "Cập nhật",
    goldUnavailable: "Tạm thời không tải được giá vàng.",
    goldViewSource: "Xem bảng giá gốc",
    goldDisclaimer: "Giá tham khảo từ Bảo Tín Mạnh Hải. Hãy xác nhận trước khi giao dịch.",
  },
} as const;
const category = (slug: string, locale: Locale) =>
  categories[slug]?.[locale] ?? slug;
const country = (code: string, fallback: string, locale: Locale) =>
  countries[code]?.[locale] ?? fallback;
const filtersFromURL = (): Filters => {
  const params = new URLSearchParams(window.location.search);
  return {
    category: params.get("category") || "",
    source: params.get("source") || "",
    country: params.get("country") || "",
    query: params.get("q") || "",
    period: params.get("period") || "",
    sort: params.get("sort") || "newest",
  };
};
const replaceFiltersURL = (filter: Filters) => {
  const url = new URL(window.location.href);
  for (const key of ["category", "source", "country", "q", "period", "sort"])
    url.searchParams.delete(key);
  if (filter.category) url.searchParams.set("category", filter.category);
  if (filter.source) url.searchParams.set("source", filter.source);
  if (filter.country) url.searchParams.set("country", filter.country);
  if (filter.query) url.searchParams.set("q", filter.query);
  if (filter.period) url.searchParams.set("period", filter.period);
  if (filter.sort && filter.sort !== "newest") url.searchParams.set("sort", filter.sort);
  window.history.replaceState(
    window.history.state,
    "",
    `${url.pathname}${url.search}${url.hash}`,
  );
};
const ago = (value: string, locale: Locale) => {
  const minutes = Math.floor((Date.now() - new Date(value).getTime()) / 60000);
  if (minutes < 60)
    return locale === "vi"
      ? `${Math.max(1, minutes)} phút`
      : `${Math.max(1, minutes)}m ago`;
  if (minutes < 1440)
    return locale === "vi"
      ? `${Math.floor(minutes / 60)} giờ`
      : `${Math.floor(minutes / 60)}h ago`;
  return new Date(value).toLocaleDateString(
    locale === "vi" ? "vi-VN" : "en-US",
    { day: "2-digit", month: "short" },
  );
};
const publishedOn = (value: string, locale: Locale) =>
  new Intl.DateTimeFormat(locale === "vi" ? "vi-VN" : "en-US", {
    day: "2-digit",
    month: "long",
    year: "numeric",
  }).format(new Date(value));
const open = (url: string) =>
  window.Telegram?.WebApp
    ? window.Telegram.WebApp.openLink(url)
    : window.open(url, "_blank", "noopener,noreferrer");
const articlePath = (article: Article) =>
  `/news/${article.id}-${
    article.title
      .toLowerCase()
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/(^-|-$)/g, "")
      .slice(0, 90) || "article"
  }`;
const articleIDFromPath = () => {
  const match = window.location.pathname.match(/^\/news\/(\d+)(?:-|$)/);
  return match ? Number(match[1]) : null;
};
const textOnly = (value: string) =>
  (value.includes("<")
    ? new DOMParser()
        .parseFromString(value, "text/html")
        .body.textContent?.trim() || ""
    : value
  )
    .replace(/\s*•\s*/g, "\n• ")
    .trim();
const safeLink = (value: string | null) => {
  try {
    const url = new URL(value || "", window.location.origin);
    return ["http:", "https:"].includes(url.protocol) ? url.href : "";
  } catch {
    return "";
  }
};
function Icon({
  name,
  filled = false,
}: {
  name:
    | "home"
    | "sparkles"
    | "bookmark"
    | "settings"
    | "filter"
    | "arrow-left"
    | "arrow-up-right"
    | "close"
    | "pen"
    | "moon"
    | "sun"
    | "history";
  filled?: boolean;
}) {
  const common = {
    fill: filled ? "currentColor" : "none",
    stroke: "currentColor",
    strokeWidth: 1.8,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  const paths = {
    home: (
      <>
        <path
          {...common}
          d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1Z"
        />
      </>
    ),
    sparkles: (
      <>
        <path
          {...common}
          d="m12 3-1.6 5.4L5 10l5.4 1.6L12 17l1.6-5.4L19 10l-5.4-1.6ZM19 15l-.7 2.3L16 18l2.3.7L19 21l.7-2.3L22 18l-2.3-.7Z"
        />
      </>
    ),
    bookmark: (
      <path
        {...common}
        d="M6 3.8A1.8 1.8 0 0 1 7.8 2h8.4A1.8 1.8 0 0 1 18 3.8V22l-6-3.6L6 22Z"
      />
    ),
    moon: <path {...common} d="M20.5 15.4A8.6 8.6 0 0 1 8.6 3.5 8.6 8.6 0 1 0 20.5 15.4Z" />,
    sun: (
      <>
        <circle {...common} cx="12" cy="12" r="4" />
        <path {...common} d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
      </>
    ),
    history: <><path {...common} d="M4 12a8 8 0 1 0 2.3-5.7L4 8.5" /><path {...common} d="M4 4v4.5h4.5M12 7v5l3 2" /></>,
    settings: (
      <>
        <circle {...common} cx="12" cy="12" r="3" />
        <path
          {...common}
          d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.1 2.1-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-3v-.2a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1-2.1-2.1.1-.1A1.7 1.7 0 0 0 7 15a1.7 1.7 0 0 0-1.6-1H5.2v-3h.2A1.7 1.7 0 0 0 7 10a1.7 1.7 0 0 0-.3-1.9l-.1-.1 2.1-2.1.1.1a1.7 1.7 0 0 0 1.9.3 1.7 1.7 0 0 0 1-1.6v-.2h3v.2a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1 2.1 2.1-.1.1A1.7 1.7 0 0 0 19.4 10a1.7 1.7 0 0 0 1.6 1h.2v3H21a1.7 1.7 0 0 0-1.6 1Z"
        />
      </>
    ),
    filter: <path {...common} d="M4 6h16M7 12h10m-7 6h4" />,
    "arrow-left": <path {...common} d="m14 6-6 6 6 6M8 12h12" />,
    "arrow-up-right": <path {...common} d="M7 17 17 7M9 7h8v8" />,
    close: <path {...common} d="m6 6 12 12M18 6 6 18" />,
    pen: (
      <path
        {...common}
        d="m4 20 4.2-1 10-10a2.8 2.8 0 0 0-4-4l-10 10L4 20Zm8.8-13.8 4 4"
      />
    ),
  };
  return (
    <svg className={`icon icon-${name}`} viewBox="0 0 24 24" aria-hidden="true">
      {paths[name]}
    </svg>
  );
}
function FormattedDescription({ value }: { value: string }) {
  const doc = new DOMParser().parseFromString(value, "text/html");
  const render = (node: Node, key: string): ReactNode => {
    if (node.nodeType === Node.TEXT_NODE) return node.textContent;
    if (node.nodeType !== Node.ELEMENT_NODE) return null;
    const element = node as HTMLElement;
    const children = Array.from(element.childNodes).map((child, index) =>
      render(child, `${key}-${index}`),
    );
    if (element.tagName === "BR") return <br key={key} />;
    if (element.tagName === "HR")
      return <hr className="detail-rule" key={key} />;
    if (element.tagName === "P")
      return (
        <p className="detail-paragraph" key={key}>
          {children}
        </p>
      );
    if (element.tagName === "A") {
      const href = safeLink(element.getAttribute("href"));
      return href ? (
        <a
          className="detail-link"
          href={href}
          onClick={(event) => {
            event.preventDefault();
            open(href);
          }}
          key={key}
        >
          {children}
        </a>
      ) : (
        <span key={key}>{children}</span>
      );
    }
    return <span key={key}>{children}</span>;
  };
  return (
    <div className="detail-description">
      {Array.from(doc.body.childNodes).map((node, index) =>
        render(node, String(index)),
      )}
    </div>
  );
}
function AIFormattedText({ value }: { value: string }) {
  return (
    <>
      {value.split(/(\*\*[^*]+\*\*)/g).map((part, index) =>
        part.startsWith("**") && part.endsWith("**") ? (
          <strong key={index}>{part.slice(2, -2)}</strong>
        ) : (
          part
        ),
      )}
    </>
  );
}
function SummaryContent({
  value,
  fallback,
}: {
  value: string;
  fallback: string;
}) {
  const content = textOnly(value);
  if (!content) return <p className="summary-empty">{fallback}</p>;
  const points = content
    .split("•")
    .map((point) => point.trim())
    .filter(Boolean);
  return points.length > 1 ? (
    <ul className="summary-list">
      {points.map((point, index) => (
        <li key={index}>
          <AIFormattedText value={point} />
        </li>
      ))}
    </ul>
  ) : (
    <p className="summary-text">
      <AIFormattedText value={content} />
    </p>
  );
}
function RSSDescription({
  description,
  locale,
}: {
  description: string;
  locale: Locale;
}) {
  if (!description) return null;
  return (
    <section className="article-content">
      <small>{locale === "vi" ? "MÔ TẢ" : "DESCRIPTION"}</small>
      <FormattedDescription value={description} />
    </section>
  );
}
type DropdownOption = { value: string; label: string };
function Dropdown({
  value,
  options,
  onChange,
  label,
  className = "",
}: {
  value: string;
  options: DropdownOption[];
  onChange: (value: string) => void;
  label: string;
  className?: string;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const selected =
    options.find((option) => option.value === value) ?? options[0];
  useEffect(() => {
    if (!isOpen) return;
    const close = (event: MouseEvent) => {
      if (!root.current?.contains(event.target as Node)) setIsOpen(false);
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [isOpen]);
  return (
    <div
      className={`dropdown ${isOpen ? "open" : ""} ${className}`}
      ref={root}
      onKeyDown={(event) => {
        if (event.key === "Escape") setIsOpen(false);
      }}
    >
      <button
        type="button"
        className="dropdown-trigger"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        onClick={() => setIsOpen((current) => !current)}
      >
        {selected?.label}
        <span className="dropdown-chevron" aria-hidden="true" />
      </button>
      {isOpen && (
        <div className="dropdown-menu" role="listbox" aria-label={label}>
          {options.map((option) => (
            <button
              type="button"
              role="option"
              aria-selected={option.value === value}
              className={option.value === value ? "selected" : ""}
              onClick={() => {
                onChange(option.value);
                setIsOpen(false);
              }}
              key={option.value}
            >
              {option.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
function LanguagePicker({
  locale,
  setLocale,
}: {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}) {
  return (
    <Dropdown
      className="language-picker"
      value={locale}
      onChange={(value) => setLocale(value as Locale)}
      label="Language"
      options={[
        { value: "en", label: "English" },
        { value: "vi", label: "Tiếng Việt" },
      ]}
    />
  );
}
function ThemeToggle({ theme, toggle, locale }: { theme: Theme; toggle: () => void; locale: Locale }) {
  const t = text[locale];
  const isDark = theme === "dark";
  return (
    <button
      type="button"
      className="theme-toggle"
      aria-label={isDark ? t.switchToLight : t.switchToDark}
      aria-pressed={isDark}
      onClick={toggle}
    >
      <Icon name={isDark ? "sun" : "moon"} />
    </button>
  );
}
const goldPrice = (value: number) =>
  new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 0 }).format(value);
const goldTime = (value: string) => value.replace(/\.\d+$/, "");
function GoldRates({ locale }: { locale: Locale }) {
  const [rates, setRates] = useState<GoldRate[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [reload, setReload] = useState(0);
  const t = text[locale];
  useEffect(() => {
    let active = true;
    const load = () => {
      api
        .goldRates()
        .then((next) => {
          if (!active) return;
          setRates(next);
          setFailed(false);
        })
        .catch(() => {
          if (active) setFailed(true);
        })
        .finally(() => {
          if (active) setLoading(false);
        });
    };
    load();
    const refresh = window.setInterval(load, 60_000);
    return () => {
      active = false;
      window.clearInterval(refresh);
    };
  }, [reload]);
  const updated = rates[0]?.last_updated;
  return (
    <section className="gold-rates" aria-labelledby="gold-rates-title">
      <div className="gold-rates-heading">
        <div>
          <small>{t.goldLive}</small>
          <h2 id="gold-rates-title">{t.goldPrices}</h2>
        </div>
        <a
          className="gold-source-link"
          href="https://www.vang.today/"
          onClick={(event) => {
            event.preventDefault();
            open("https://www.vang.today/");
          }}
        >
          {t.goldViewSource}
          <Icon name="arrow-up-right" />
        </a>
      </div>
      {loading ? (
        <div className="gold-rates-loading" role="status">
          <span className="loading-spinner" aria-hidden="true" />
          {locale === "vi" ? "Đang tải giá vàng…" : "Loading gold prices…"}
        </div>
      ) : failed && !rates.length ? (
        <p className="gold-rates-error" role="alert">
          {t.goldUnavailable}{" "}
          <button className="text-button" onClick={() => setReload((value) => value + 1)}>
            {t.retry}
          </button>
        </p>
      ) : (
        <>
          <div className="gold-rates-grid">
            {rates.map((rate) => (
              <article className="gold-rate" key={rate.code}>
                <p className="gold-rate-name">{rate.name}</p>
                <p className="gold-rate-unit">{rate.unit}</p>
                <dl>
                  <div>
                    <dt>{t.goldBuy}</dt>
                    <dd>{goldPrice(rate.buy_price)}</dd>
                  </div>
                  <div>
                    <dt>{t.goldSell}</dt>
                    <dd>{goldPrice(rate.sell_price)}</dd>
                  </div>
                </dl>
                {rate.trend !== "neutral" && rate.trend_value && (
                  <p className={`gold-rate-trend ${rate.trend}`}>
                    {rate.trend === "up"
                      ? locale === "vi"
                        ? "Tăng"
                        : "Up"
                      : locale === "vi"
                        ? "Giảm"
                        : "Down"}{" "}
                    {rate.trend_value.replace(/^-/, "")} ₫
                  </p>
                )}
              </article>
            ))}
          </div>
          <p className="gold-rates-meta">
            {updated && <span>{t.goldUpdated}: {goldTime(updated)}</span>}
            <span>{t.goldDisclaimer}</span>
          </p>
        </>
      )}
    </section>
  );
}
function Card({
  article,
  hero,
  locale,
  toggle,
  openDetail,
  selected,
  select,
}: {
  article: Article;
  hero?: boolean;
  locale: Locale;
  toggle: (article: Article) => void;
  openDetail: (article: Article) => void;
  selected?: boolean;
  select?: (article: Article) => void;
}) {
  const t = text[locale];
  return (
    <article className={hero ? "hero-card" : "news-card"}>
      <button
        type="button"
        className="card-open"
        onClick={() => openDetail(article)}
        aria-label={`${t.openArticle}: ${article.title}`}
      >
        {article.image_url ? (
          <img src={article.image_url} alt="" />
        ) : (
          <div className="cover-placeholder">SIGNAL</div>
        )}
        <div className="card-copy">
          <div className="eyebrow">
            {article.source}
            <span aria-hidden="true"> · </span>
            {country(article.country_code, article.country_name, locale)}
            <span aria-hidden="true"> · </span>
            {ago(article.published_at, locale)}
          </div>
          <h2>{article.title}</h2>
          {article.summary && <p>{textOnly(article.summary)}</p>}
          <span className={`category-label category-${article.category}`}>
            {category(article.category, locale)}
          </span>
        </div>
      </button>
      <a
        className="source-citation card-source"
        href={article.url}
        onClick={(event) => {
          event.preventDefault();
          open(article.url);
        }}
      >
        {t.source}: {article.source}
        <Icon name="arrow-up-right" />
      </a>
      <button
        type="button"
        className={`save ${article.is_saved ? "active" : ""}`}
        aria-label={article.is_saved ? t.unsave : t.save}
        aria-pressed={article.is_saved}
        onClick={() => toggle(article)}
      >
        <Icon name="bookmark" filled={article.is_saved} />
      </button>
      {select && <label className="save"><input type="checkbox" checked={selected} onChange={() => select(article)} aria-label={`${t.selectSaved}: ${article.title}`} /></label>}
    </article>
  );
}
function Detail({
  article,
  locale,
  back,
}: {
  article: Article;
  locale: Locale;
  back: () => void;
}) {
  const [translation, setTranslation] = useState<Translation | null>(null);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [manualBrief, setManualBrief] = useState<Pick<
    Article,
    "title" | "summary"
  > | null>(null);
  const [resummarizing, setResummarizing] = useState(false);
  const [resummarizeFailed, setResummarizeFailed] = useState(false);
  const [isRead, setIsRead] = useState(article.is_read);
  const [feedbackIssue, setFeedbackIssue] = useState<"incorrect" | "missing" | null>(null);
  const [feedbackReason, setFeedbackReason] = useState("");
  const [feedbackSent, setFeedbackSent] = useState(false);
  const [feedbackFailed, setFeedbackFailed] = useState(false);
  const t = text[locale];
  const translate = () => {
    if (locale !== "vi") return;
    setLoading(true);
    setFailed(false);
    api
      .translateVietnamese(article.id)
      .then(setTranslation)
      .catch(() => setFailed(true))
      .finally(() => setLoading(false));
  };
  const resummarize = async () => {
    setResummarizing(true);
    setResummarizeFailed(false);
    try {
      const brief = await api.resummarize(article.id);
      setManualBrief(brief);
      setTranslation(null);
      setFailed(false);
    } catch {
      setResummarizeFailed(true);
    } finally {
      setResummarizing(false);
    }
  };
  useEffect(() => {
    setTranslation(null);
    setFailed(false);
    setManualBrief(null);
    setResummarizeFailed(false);
    setIsRead(article.is_read);
    setFeedbackIssue(null);
    setFeedbackReason("");
    setFeedbackSent(false);
    setFeedbackFailed(false);
    if (locale === "vi") translate();
  }, [article.id, locale]);
  useEffect(() => {
    void api.startReading(article.id);
  }, [article.id]);
  const title = translation?.title || manualBrief?.title || article.title;
  const summary =
    translation?.summary || manualBrief?.summary || article.summary;
  const originalDetail = article.description || article.summary;
  const aiNotice =
    locale === "vi"
      ? "Đoạn tóm tắt này được tạo bởi AI và có thể chứa thông tin không chính xác."
      : "This summary was generated by AI and may contain inaccuracies.";
  const resummarizeLabel =
    locale === "vi" ? "Tạo lại tóm tắt bằng AI" : "Generate a new AI summary";
  const contentImages =
    article.content_images?.filter((image) => image !== article.image_url) ??
    [];
  const submitFeedback = async () => {
    if (!feedbackIssue || feedbackReason.trim().length < 3) return;
    setFeedbackFailed(false);
    try {
      await api.submitAIFeedback(article.id, feedbackIssue, feedbackReason.trim());
      setFeedbackSent(true);
      setFeedbackIssue(null);
      setFeedbackReason("");
    } catch { setFeedbackFailed(true); }
  };
  return (
    <section className="page detail">
      <button className="back-button" onClick={back}>
        <Icon name="arrow-left" />
        {t.back}
      </button>
      <p className="eyebrow">
        {article.source} · {publishedOn(article.published_at, locale)}
      </p>
      <span className={`category-label category-${article.category}`}>
        {category(article.category, locale)}
      </span>
      <h1>{title}</h1>
      <a
        className="detail-citation"
        href={article.url}
        onClick={(event) => {
          event.preventDefault();
          open(article.url);
        }}
      >
        {t.source}: {article.source}
        <Icon name="arrow-up-right" />
      </a>
      {article.image_url && (
        <img className="detail-image" src={article.image_url} alt="" />
      )}
      {loading && (
        <p className="translation-loading" role="status">
          <span className="loading-spinner" aria-hidden="true" />
          {t.translating}
        </p>
      )}
      {failed && (
        <p className="state error">
          {t.translationError}{" "}
          <button className="text-button" onClick={translate}>
            {t.retry}
          </button>
        </p>
      )}
      {
        <section className="summary-panel">
          <div className="summary-heading">
            <small>{t.summary}</small>
            <button
              type="button"
              className="resummarize-button"
              aria-label={resummarizeLabel}
              title={resummarizeLabel}
              onClick={resummarize}
              disabled={resummarizing}
            >
              {resummarizing ? (
                <span className="loading-spinner" aria-hidden="true" />
              ) : (
                <Icon name="sparkles" />
              )}
            </button>
          </div>
          <SummaryContent value={summary} fallback={t.summaryEmpty} />
          {resummarizeFailed && (
            <p className="resummarize-error" role="alert">
              {locale === "vi"
                ? "Không thể tạo lại tóm tắt. Vui lòng thử lại."
                : "Could not generate a new summary. Please try again."}
            </p>
          )}
          <p className="ai-notice">{aiNotice}</p>
          <div className="ai-feedback" aria-label={locale === "vi" ? "Phản hồi chất lượng AI" : "AI quality feedback"}>
            <button className="text-button" onClick={() => { setFeedbackIssue("incorrect"); setFeedbackSent(false); }}>{t.feedbackIncorrect}</button>
            <button className="text-button" onClick={() => { setFeedbackIssue("missing"); setFeedbackSent(false); }}>{t.feedbackMissing}</button>
          </div>
          {feedbackIssue && <div className="ai-feedback-form"><label>{t.feedbackReason}<textarea value={feedbackReason} onChange={event => setFeedbackReason(event.target.value)} rows={3} maxLength={2000} /></label><button className="text-button" disabled={feedbackReason.trim().length < 3} onClick={() => void submitFeedback()}>{t.sendFeedback}</button>{feedbackFailed && <p className="state error">{t.loadError}</p>}</div>}
          {feedbackSent && <p className="ai-notice">{t.feedbackThanks}</p>}
        </section>
      }
      <details className="article-content"><summary>{t.originalContent}</summary><RSSDescription description={article.original_content || article.description} locale={locale} /></details>
      {!isRead && (
        <button className="text-button" onClick={() => api.markRead(article.id).then(() => setIsRead(true)).catch(() => {})}>
          {t.markRead}
        </button>
      )}
      <button className="primary source-link" onClick={() => open(article.url)}>
        {t.readOriginal}
        <Icon name="arrow-up-right" />
      </button>
    </section>
  );
}
function Home({
  saved,
  history,
  locale,
  setLocale,
  theme,
  toggleTheme,
  openDetail,
}: {
  saved?: boolean;
  history?: boolean;
  locale: Locale;
  setLocale: (locale: Locale) => void;
  theme: Theme;
  toggleTheme: () => void;
  openDetail: (article: Article) => void;
}) {
  const [items, setItems] = useState<Article[]>([]);
  const [cats, setCats] = useState<Category[]>([]);
  const [sources, setSources] = useState<Source[]>([]);
  const [countryList, setCountryList] = useState<Country[]>([]);
  const [filter, setFilter] = useState<Filters>(filtersFromURL);
  const [search, setSearch] = useState(() => filter.query);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState(false);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(true);
  const [reload, setReload] = useState(0);
  const [hideRead, setHideRead] = useState(false);
  const [savedOrganization, setSavedOrganization] = useState<SavedOrganization>({ folders: [], tags: [] });
  const [savedFolder, setSavedFolder] = useState("");
  const [savedTag, setSavedTag] = useState("");
  const [savedSort, setSavedSort] = useState("saved_newest");
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const t = text[locale];
  const params = useMemo(
    () =>
      new URLSearchParams({
        limit: "30",
        offset: String(offset),
        lang: locale,
        ...(filter.category && { category: filter.category }),
        ...(filter.source && { source: filter.source }),
        ...(filter.country && { country: filter.country }),
        ...(filter.query && { q: filter.query }),
        ...(!saved && filter.period && { period: filter.period }),
        ...(!saved && filter.sort && { sort: filter.sort }),
        ...(hideRead && { hide_read: "1" }),
        ...(saved && savedFolder && { folder: savedFolder }),
        ...(saved && savedTag && { tag: savedTag }),
        ...(saved && { sort: savedSort }),
      }),
    [filter, hideRead, locale, offset, saved, savedFolder, savedTag, savedSort],
  );
  useEffect(() => {
    const timer = window.setTimeout(
      () =>
        setFilter((current) =>
          current.query === search ? current : { ...current, query: search },
        ),
      300,
    );
    return () => window.clearTimeout(timer);
  }, [search]);
  useEffect(() => {
    if (!saved && !history) replaceFiltersURL(filter);
  }, [filter, saved, history]);
  useEffect(() => {
    if (saved || history) return;
    const syncFilters = () => {
      const next = filtersFromURL();
      setFilter((current) =>
        current.category === next.category &&
        current.source === next.source &&
        current.country === next.country &&
        current.query === next.query &&
        current.period === next.period &&
        current.sort === next.sort
          ? current
          : next,
      );
      setSearch(next.query);
    };
    window.addEventListener("popstate", syncFilters);
    return () => window.removeEventListener("popstate", syncFilters);
  }, [saved, history]);
  useEffect(() => {
    if (!history)
      Promise.all([api.categories(), api.sources(), api.countries()])
        .then(([a, b, c]) => {
          setCats(a);
          setSources(b);
          setCountryList(c);
        })
        .catch(() => {});
  }, [saved, history]);
  useEffect(() => {
    if (saved) api.savedOrganization().then(setSavedOrganization).catch(() => {});
  }, [saved, reload]);
  useEffect(() => {
    setOffset(0);
    setHasMore(true);
  }, [filter, locale, saved, history, savedFolder, savedTag, savedSort]);
  useEffect(() => {
    let active = true;
    if (offset === 0) {
      setLoading(true);
      setError(false);
    } else setLoadingMore(true);
    (history ? api.readingHistory(locale) : saved ? api.saved(params) : api.articles(params))
      .then((next) => {
        if (!active) return;
        setItems((current) => (offset === 0 ? next : [...current, ...next]));
        setHasMore(!saved && !history && next.length === 30);
      })
      .catch(() => {
        if (active) setError(true);
      })
      .finally(() => {
        if (active) {
          setLoading(false);
          setLoadingMore(false);
        }
      });
    return () => {
      active = false;
    };
  }, [saved, history, locale, params, offset, reload]);
  useEffect(() => {
    if (saved || history) return;
    const loadMore = () => {
      if (
        !loading &&
        !loadingMore &&
        hasMore &&
        window.innerHeight + window.scrollY >=
          document.documentElement.scrollHeight - 360
      )
        setOffset((current) => current + 30);
    };
    window.addEventListener("scroll", loadMore, { passive: true });
    loadMore();
    return () => window.removeEventListener("scroll", loadMore);
  }, [saved, history, loading, loadingMore, hasMore]);
  const clearHistory = async () => {
    try {
      await api.clearReadingHistory();
      setItems([]);
    } catch {
      setError(true);
    }
  };
  const toggleSelected = (article: Article) => setSelectedIDs(current => current.includes(article.id) ? current.filter(id => id !== article.id) : [...current, article.id]);
  const removeSelected = async () => {
    if (!selectedIDs.length) return;
    try {
      await api.bulkUnsave(selectedIDs);
      setItems(current => current.filter(article => !selectedIDs.includes(article.id)));
      setSelectedIDs([]);
      setReload(value => value + 1);
    } catch { setError(true); }
  };
  const addCollection = async (type: "folder" | "tag") => {
    const name = window.prompt(type === "folder" ? t.addFolder : t.addTag);
    if (!name?.trim()) return;
    try {
      const item = type === "folder" ? await api.createSavedFolder(name) : await api.createSavedTag(name);
      setSavedOrganization(current => type === "folder" ? { ...current, folders: [...current.folders, item] } : { ...current, tags: [...current.tags, item] });
    } catch { setError(true); }
  };
  const assignCollection = async (type: "folder" | "tag", id: number) => {
    if (!selectedIDs.length || !id) return;
    const selected = items.filter(article => selectedIDs.includes(article.id));
    try {
      await Promise.all(selected.map(article => type === "folder" ? api.setSavedFolders(article.id, [...new Set([...(article.folder_ids ?? []), id])]) : api.setSavedTags(article.id, [...new Set([...(article.tag_ids ?? []), id])])));
      setReload(value => value + 1);
    } catch { setError(true); }
  };
  const toggle = async (article: Article) => {
    setItems((current) =>
      current.map((item) =>
        item.id === article.id ? { ...item, is_saved: !item.is_saved } : item,
      ),
    );
    try {
      await api.toggleSaved(article);
      if (saved && !article.is_saved)
        setItems((current) => current.filter((item) => item.id !== article.id));
    } catch {
      setItems((current) =>
        current.map((item) =>
          item.id === article.id
            ? { ...item, is_saved: article.is_saved }
            : item,
        ),
      );
    }
  };
  const hero = !saved && !history ? items[0] : undefined;
  const list = hero ? items.slice(1) : items;
  const categoryOptions = [
    { value: "", label: t.allCategories },
    ...cats.map((item) => ({
      value: item.slug,
      label: category(item.slug, locale),
    })),
  ];
  const sourceOptions = [
    { value: "", label: t.allSources },
    ...sources.map((item) => ({ value: String(item.id), label: item.name })),
  ];
  const countryOptions = [
    { value: "", label: t.allCountries },
    ...countryList.map((item) => ({
      value: item.code,
      label: country(item.code, item.name, locale),
    })),
  ];
  return (
    <section className="page editorial">
      <header className="masthead">
        <div>
          <p>{t.masthead}</p>
          <h1>{history ? t.history : saved ? t.saved : t.news}</h1>
        </div>
        <div className="header-actions">
          <LanguagePicker locale={locale} setLocale={setLocale} />
          <ThemeToggle theme={theme} toggle={toggleTheme} locale={locale} />
          {!saved && !history && <button className="text-button" onClick={() => setHideRead(value => !value)}>{hideRead ? t.showRead : t.hideRead}</button>}
        </div>
      </header>
      {!history && (
        <FilterControls
          locale={locale}
          filter={filter}
          setFilter={setFilter}
          search={search}
          setSearch={setSearch}
          countries={countryList}
          sources={sources}
          categories={cats}
        />
      )}
      {!saved && !history && <GoldRates locale={locale} />}
      {history && <button className="text-button" onClick={clearHistory}>{t.clearHistory}</button>}
      {saved && <section className="filter-controls" aria-label={t.saved}>
        <label className="filter-field"><span>{t.sortSaved}</span><select value={savedSort} onChange={event => setSavedSort(event.target.value)}><option value="saved_newest">{t.savedNewest}</option><option value="saved_oldest">{t.savedOldest}</option><option value="published_newest">{t.publishedNewest}</option><option value="title">{t.titleSort}</option></select></label>
        <label className="filter-field"><span>{t.folders}</span><select value={savedFolder} onChange={event => setSavedFolder(event.target.value)}><option value="">{t.all}</option>{savedOrganization.folders.map(folder => <option key={folder.id} value={folder.id}>{folder.name} ({folder.count})</option>)}</select></label>
        <label className="filter-field"><span>{t.tags}</span><select value={savedTag} onChange={event => setSavedTag(event.target.value)}><option value="">{t.all}</option>{savedOrganization.tags.map(tag => <option key={tag.id} value={tag.id}>{tag.name} ({tag.count})</option>)}</select></label>
        <button className="text-button" onClick={() => void addCollection("folder")}>{t.addFolder}</button><button className="text-button" onClick={() => void addCollection("tag")}>{t.addTag}</button>
        {selectedIDs.length > 0 && <><button className="text-button" onClick={() => void removeSelected()}>{t.removeSelected} ({selectedIDs.length})</button><select defaultValue="" onChange={event => { void assignCollection("folder", Number(event.target.value)); event.currentTarget.value = ""; }}><option value="">{t.folders}</option>{savedOrganization.folders.map(folder => <option key={folder.id} value={folder.id}>{folder.name}</option>)}</select><select defaultValue="" onChange={event => { void assignCollection("tag", Number(event.target.value)); event.currentTarget.value = ""; }}><option value="">{t.tags}</option>{savedOrganization.tags.map(tag => <option key={tag.id} value={tag.id}>{tag.name}</option>)}</select></>}
      </section>}
      {loading ? (
        <p className="state" role="status">
          {t.loading}
        </p>
      ) : error && !items.length ? (
        <p className="state error">
          {t.loadError}{" "}
          <button
            className="text-button"
            onClick={() => setReload((value) => value + 1)}
          >
            {t.retry}
          </button>
        </p>
      ) : (
        <>
          {hero && (
            <>
              <p className="section-kicker">{t.featured}</p>
              <Card
                hero
                article={hero}
                locale={locale}
                toggle={toggle}
                openDetail={openDetail}
              />
            </>
          )}
          <div className="feed">
            <div className="section-head">
              <h3>{history ? t.history : saved ? t.yourArticles : t.latest}</h3>
              <span>
                {list.length} {t.articles}
              </span>
            </div>
            {list.map((item) => (
              <Card
                article={item}
                locale={locale}
                toggle={toggle}
                openDetail={openDetail}
                selected={selectedIDs.includes(item.id)}
                select={saved ? toggleSelected : undefined}
                key={item.id}
              />
            ))}
            {!items.length && <p className="state">{t.empty}</p>}
            {error && items.length > 0 && (
              <p className="state error">
                {t.loadError}{" "}
                <button
                  className="text-button"
                  onClick={() => setReload((value) => value + 1)}
                >
                  {t.retry}
                </button>
              </p>
            )}
            {loadingMore && (
              <p className="state loading-more" role="status">
                {t.loadingMore}
              </p>
            )}
          </div>
        </>
      )}
    </section>
  );
}
function Featured({
  locale,
  setLocale,
  openDetail,
}: {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  openDetail: (article: Article) => void;
}) {
  const [brief, setBrief] = useState<FeaturedBrief | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [reload, setReload] = useState(0);
  const t = text[locale];
  useEffect(() => {
    let active = true;
    setLoading(true);
    setFailed(false);
    api
      .featured(locale)
      .then((value) => {
        if (active) setBrief(value);
      })
      .catch(() => {
        if (active) {
          setBrief(null);
          setFailed(true);
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [locale, reload]);
  const toggle = async (article: Article) => {
    setBrief((current) =>
      current
        ? {
            ...current,
            topics: current.topics.map((topic) => ({
              ...topic,
              articles: topic.articles.map((item) =>
                item.id === article.id
                  ? { ...item, is_saved: !item.is_saved }
                  : item,
              ),
            })),
          }
        : current,
    );
    try {
      await api.toggleSaved(article);
    } catch {
      setBrief((current) =>
        current
          ? {
              ...current,
              topics: current.topics.map((topic) => ({
                ...topic,
                articles: topic.articles.map((item) =>
                  item.id === article.id
                    ? { ...item, is_saved: article.is_saved }
                    : item,
                ),
              })),
            }
          : current,
      );
    }
  };
  return (
    <section className="page featured-page">
      <header className="masthead">
        <div>
          <p>{t.masthead}</p>
          <h1>{t.featuredNews}</h1>
        </div>
        <LanguagePicker locale={locale} setLocale={setLocale} />
      </header>
      {loading ? (
        <p className="state" role="status">
          {t.loading}
        </p>
      ) : failed ? (
        <p className="state error">
          {t.loadError}{" "}
          <button
            className="text-button"
            onClick={() => setReload((value) => value + 1)}
          >
            {t.retry}
          </button>
        </p>
      ) : !brief ? (
        <p className="state">{t.featuredUnavailable}</p>
      ) : (
        <>
          <section className="featured-lead">
            <p className="section-kicker">
              {t.featuredUpdated} · {publishedOn(brief.generated_at, locale)}
            </p>
            <h2>{brief.title}</h2>
            <p>{brief.intro}</p>
          </section>
          <div className="featured-topics">
            {brief.topics.map((topic) => (
              <FeaturedTopic
                topic={topic}
                locale={locale}
                toggle={toggle}
                openDetail={openDetail}
                key={topic.id}
              />
            ))}
          </div>
        </>
      )}
    </section>
  );
}
function FeaturedTopic({
  topic,
  locale,
  toggle,
  openDetail,
}: {
  topic: FeaturedBrief["topics"][number];
  locale: Locale;
  toggle: (article: Article) => void;
  openDetail: (article: Article) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const visibleArticles = expanded
    ? topic.articles
    : topic.articles.slice(0, 4);
  const remaining = topic.articles.length - visibleArticles.length;
  const buttonLabel =
    locale === "vi"
      ? expanded
        ? "Thu gọn"
        : `Xem thêm ${remaining} tin`
      : expanded
        ? "Show less"
        : `Show ${remaining} more`;
  return (
    <section className="featured-topic">
      <span className="category-label">
        {String(topic.position + 1).padStart(2, "0")}
      </span>
      <h2>{topic.title}</h2>
      <p>{topic.summary}</p>
      <div className="featured-articles">
        {visibleArticles.map((article) => (
          <Card
            article={article}
            locale={locale}
            toggle={toggle}
            openDetail={openDetail}
            key={article.id}
          />
        ))}
      </div>
      {topic.articles.length > 4 && (
        <button
          type="button"
          className="show-more-button"
          aria-expanded={expanded}
          onClick={() => setExpanded((value) => !value)}
        >
          {buttonLabel}
          <Icon name={expanded ? "arrow-left" : "arrow-up-right"} />
        </button>
      )}
    </section>
  );
}
function Settings({
  locale,
  setLocale,
}: {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}) {
  const t = text[locale];
  return (
    <section className="page settings">
      <h1>{t.settings}</h1>
      <div className="settings-card">
        <small>{t.language}</small>
        <LanguagePicker locale={locale} setLocale={setLocale} />
      </div>
      <div className="settings-card">
        <small>{t.interface}</small>
        <p>{t.theme}</p>
      </div>
    </section>
  );
}
export default function App() {
  const telegramLocale =
    window.Telegram?.WebApp?.initDataUnsafe?.user?.language_code === "vi"
      ? "vi"
      : "en";
  const [locale, setLocaleState] = useState<Locale>(
    () => (localStorage.getItem("locale") as Locale) || telegramLocale,
  );
  const [themeOverride, setThemeOverride] = useState<Theme | null>(() => {
    const saved = localStorage.getItem("theme");
    return saved === "light" || saved === "dark" ? saved : null;
  });
  const [automaticTheme, setAutomaticTheme] = useState<Theme>(() =>
    window.Telegram?.WebApp?.colorScheme ??
    (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"),
  );
  const [tab, setTab] = useState<Tab>("home");
  const [article, setArticle] = useState<Article | null>(null);
  const [articleID, setArticleID] = useState<number | null>(articleIDFromPath);
  const [detailError, setDetailError] = useState(false);
  const [detailReload, setDetailReload] = useState(0);
  const listingScroll = useRef(0);
  const setLocale = (next: Locale) => {
    localStorage.setItem("locale", next);
    setLocaleState(next);
  };
  const t = text[locale];
  const theme = themeOverride ?? automaticTheme;
  const toggleTheme = () => {
    const next = theme === "dark" ? "light" : "dark";
    localStorage.setItem("theme", next);
    setThemeOverride(next);
  };
  useEffect(() => {
    const app = window.Telegram?.WebApp;
    const systemTheme = window.matchMedia("(prefers-color-scheme: dark)");
    const applyTheme = () => {
      setAutomaticTheme(app?.colorScheme ?? (systemTheme.matches ? "dark" : "light"));
    };
    applyTheme();
    if (app) {
      app.ready();
      app.expand();
      app.onEvent("themeChanged", applyTheme);
      return () => app.offEvent("themeChanged", applyTheme);
    }
    systemTheme.addEventListener("change", applyTheme);
    return () => systemTheme.removeEventListener("change", applyTheme);
  }, []);
  useEffect(() => {
    document.documentElement.dataset.telegramTheme = theme;
  }, [theme]);
  useEffect(() => {
    const onPopState = () => setArticleID(articleIDFromPath());
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);
  useEffect(() => {
    if (articleID === null) {
      setArticle(null);
      setDetailError(false);
      return;
    }
    setArticle((current) => (current?.id === articleID ? current : null));
    setDetailError(false);
    let active = true;
    api
      .article(articleID)
      .then((item) => {
        if (active) setArticle(item);
      })
      .catch(() => {
        if (active) setDetailError(true);
      });
    return () => {
      active = false;
    };
  }, [articleID, detailReload]);
  const openDetail = (next: Article) => {
    listingScroll.current = window.scrollY;
    window.history.pushState({}, "", articlePath(next));
    setArticle(next);
    setArticleID(next.id);
    window.scrollTo(0, 0);
  };
  const back = () => {
    if (window.history.state !== null) window.history.back();
    else {
      window.history.replaceState({}, "", "/");
      setArticleID(null);
    }
    requestAnimationFrame(() => window.scrollTo(0, listingScroll.current));
  };
  const showingDetail = articleID !== null;
  const navItems: [Tab, Parameters<typeof Icon>[0]["name"], string][] = [
    ["home", "home", t.home],
    ["featured", "sparkles", t.featuredNews],
    ["saved", "bookmark", t.saved],
    ["history", "history", t.history],
    ["settings", "settings", t.settings],
  ];
  return (
    <main>
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <div className="app-shell" id="main-content" tabIndex={-1}>
        <div
          className={showingDetail ? "listing-page hidden" : "listing-page"}
          aria-hidden={showingDetail}
        >
          {tab === "home" && (
            <Home
              locale={locale}
              setLocale={setLocale}
              theme={theme}
              toggleTheme={toggleTheme}
              openDetail={openDetail}
            />
          )}
          {tab === "featured" && (
            <Featured
              locale={locale}
              setLocale={setLocale}
              openDetail={openDetail}
            />
          )}
          {tab === "saved" && (
            <Home
              saved
              locale={locale}
              setLocale={setLocale}
              theme={theme}
              toggleTheme={toggleTheme}
              openDetail={openDetail}
            />
          )}
          {tab === "history" && (
            <Home
              history
              locale={locale}
              setLocale={setLocale}
              theme={theme}
              toggleTheme={toggleTheme}
              openDetail={openDetail}
            />
          )}
          {tab === "settings" && (
            <Settings locale={locale} setLocale={setLocale} />
          )}
        </div>
        {showingDetail &&
          (article ? (
            <Detail article={article} locale={locale} back={back} />
          ) : detailError ? (
            <section className="state error">
              <p>{t.detailError}</p>
              <button
                className="primary"
                onClick={() => setDetailReload((value) => value + 1)}
              >
                {t.retry}
              </button>
              <button className="text-button detail-back" onClick={back}>
                {t.back}
              </button>
            </section>
          ) : (
            <p className="state" role="status">
              {t.loading}
            </p>
          ))}
      </div>
      {!showingDetail && (
        <nav aria-label="Primary navigation">
          {navItems.map(([id, icon, label]) => (
            <button
              className={tab === id ? "nav-active" : ""}
              aria-current={tab === id ? "page" : undefined}
              onClick={() => setTab(id)}
              key={id}
            >
              <Icon name={icon} filled={id === "saved" && tab === id} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
      )}
    </main>
  );
}
