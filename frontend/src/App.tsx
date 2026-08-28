import { memo, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { api } from "./api";
import "./featured.css";
import { FilterControls, FilterIcon, FilterSelect } from "./filter-controls";
import type {
  Article,
  AIConfigInput,
  AIModel,
  Category,
  Country,
  FeaturedBrief,
  GoldRate,
  MediumReaderArticle,
  SavedOrganization,
  Source,
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
    originalContent: "Related original text",
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
    featured: "TODAY",
    featuredNews: "Today",
    featuredUpdated: "Updated through",
    featuredUnavailable: "Today's briefing is not available yet.",
    featuredTakeaways: "3 things to know",
    featuredWhyItMatters: "Why it matters",
    featuredCoverage: "Last 24 hours",
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
    share: "Share article",
    save: "Save article",
    unsave: "Remove saved article",
    translating: "Translating to Vietnamese…",
    translationError:
      "Could not translate this article. Showing the original text.",
    retry: "Try again",
    language: "Language",
    summary: "SUMMARY",
    summaryReadingTime: "{minutes} min summary read",
    readMode: "Reading mode",
    exitReadMode: "Exit reading mode",
    scrollToTop: "Scroll to top",
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
    mediumReader: "Medium Reader",
    mediumReaderHint: "Paste a Medium link to read the public version here.",
    mediumURL: "Medium article URL",
    mediumOpen: "Open article",
    mediumLoading: "Fetching public article content…",
    mediumPublic: "Public article",
    mediumFree: "Author-provided free link",
    mediumPreview: "Public preview only",
    mediumPreviewWarning: "Medium did not provide the complete public article. Open the original to continue.",
    mediumOriginal: "Open original on Medium",
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
    originalContent: "NGUYÊN VĂN LIÊN QUAN",
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
    featured: "HÔM NAY",
    featuredNews: "Hôm nay",
    featuredUpdated: "Cập nhật đến",
    featuredUnavailable: "Bản tin hôm nay chưa sẵn sàng.",
    featuredTakeaways: "3 điều cần biết",
    featuredWhyItMatters: "Vì sao đáng chú ý",
    featuredCoverage: "24 giờ qua",
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
    share: "Chia sẻ bài viết",
    save: "Lưu bài viết",
    unsave: "Bỏ lưu bài viết",
    translating: "Đang dịch sang tiếng Việt…",
    translationError: "Không thể dịch bài này. Đang hiển thị nội dung gốc.",
    retry: "Thử lại",
    language: "Ngôn ngữ",
    summary: "TÓM TẮT",
    summaryReadingTime: "Đọc tóm tắt {minutes} phút",
    readMode: "Chế độ đọc",
    exitReadMode: "Thoát chế độ đọc",
    scrollToTop: "Lên đầu trang",
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
    mediumReader: "Đọc Medium",
    mediumReaderHint: "Dán link Medium để đọc phiên bản công khai tại đây.",
    mediumURL: "Link bài viết Medium",
    mediumOpen: "Mở bài viết",
    mediumLoading: "Đang tải nội dung công khai…",
    mediumPublic: "Bài viết công khai",
    mediumFree: "Liên kết miễn phí từ tác giả",
    mediumPreview: "Chỉ có bản xem trước công khai",
    mediumPreviewWarning: "Medium không trả toàn bộ nội dung công khai. Mở bài gốc để tiếp tục.",
    mediumOriginal: "Mở bài gốc trên Medium",
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
const shareArticle = async (article: Article) => {
  const url = new URL(articlePath(article), window.location.origin).href;
  const telegramShareURL = `https://t.me/share/url?url=${encodeURIComponent(url)}&text=${encodeURIComponent(article.title)}`;
  if (window.Telegram?.WebApp?.openTelegramLink) {
    window.Telegram.WebApp.openTelegramLink(telegramShareURL);
    return;
  }
  if (navigator.share) {
    try {
      await navigator.share({ title: article.title, text: article.title, url });
      return;
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
    }
  }
  window.open(telegramShareURL, "_blank", "noopener,noreferrer");
};
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
    | "arrow-up"
    | "arrow-up-right"
    | "close"
    | "pen"
    | "moon"
    | "sun"
    | "history"
    | "share"
    | "power"
    | "play";
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
    share: <><circle {...common} cx="18" cy="5" r="2.5" /><circle {...common} cx="6" cy="12" r="2.5" /><circle {...common} cx="18" cy="19" r="2.5" /><path {...common} d="m8.2 10.8 7.6-4.6m-7.6 7 7.6 4.6" /></>,
    power: <><path {...common} d="M12 3v9" /><path {...common} d="M7.1 5.9a8 8 0 1 0 9.8 0" /></>,
    play: <path {...common} d="m9 6 9 6-9 6Z" />,
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
    "arrow-up": <path {...common} d="m6 14 6-6 6 6M12 6v12" />,
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
const lightboxConfigPattern = /\{\s*"lightbox_close"[\s\S]*?"lightbox_toggle_sidebar"\s*:\s*"[^"]*"\s*\}/g;

const cleanOriginalText = (value: string) =>
  value
    .replace(lightboxConfigPattern, "")
    .replace(/[\u200B-\u200D\uFEFF]/g, "")
    .replace(/\n[ \t]+\n/g, "\n\n")
    .trim();

function PlainTextParagraphs({ value }: { value: string }) {
  return (
    <>
      {cleanOriginalText(value)
        .split(/\n\s*\n/)
        .map((paragraph) => paragraph.replace(/\s*\n\s*/g, " ").trim())
        .filter(Boolean)
        .map((paragraph, index) => (
          <p className="detail-paragraph" key={index}>
            <AIFormattedText value={paragraph} />
          </p>
        ))}
    </>
  );
}

function FormattedDescription({ value }: { value: string }) {
  const cleanedValue = cleanOriginalText(value);
  if (!cleanedValue) return null;
  if (!cleanedValue.includes("<")) {
    return <div className="detail-description"><PlainTextParagraphs value={cleanedValue} /></div>;
  }
  const doc = new DOMParser().parseFromString(cleanedValue, "text/html");
  const render = (node: Node, key: string): ReactNode => {
    if (node.nodeType === Node.TEXT_NODE) return node.textContent;
    if (node.nodeType !== Node.ELEMENT_NODE) return null;
    const element = node as HTMLElement;
    if (["SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE"].includes(element.tagName)) return null;
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
const estimateReadingMinutes = (value: string) => {
  const words = textOnly(value).trim().split(/\s+/).filter(Boolean).length;
  return Math.max(1, Math.ceil(words / 180));
};
function RSSDescription({ description }: { description: string }) {
  if (!description) return null;
  return <FormattedDescription value={description} />;
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
const HomeMasthead = memo(function HomeMasthead({
  saved,
  history,
  locale,
  setLocale,
  theme,
  toggleTheme,
  hideRead,
  toggleHideRead,
}: {
  saved?: boolean;
  history?: boolean;
  locale: Locale;
  setLocale: (locale: Locale) => void;
  theme: Theme;
  toggleTheme: () => void;
  hideRead: boolean;
  toggleHideRead: () => void;
}) {
  const t = text[locale];
  return (
    <header className="masthead">
      <div>
        <p>{t.masthead}</p>
        <h1>{history ? t.history : saved ? t.saved : t.news}</h1>
      </div>
      <div className="header-actions">
        <LanguagePicker locale={locale} setLocale={setLocale} />
        <ThemeToggle theme={theme} toggle={toggleTheme} locale={locale} />
        {!saved && !history && <button className="text-button" onClick={toggleHideRead}>{hideRead ? t.showRead : t.hideRead}</button>}
      </div>
    </header>
  );
});
const FeaturedMasthead = memo(function FeaturedMasthead({ locale, setLocale }: { locale: Locale; setLocale: (locale: Locale) => void }) {
  const t = text[locale];
  return <header className="masthead"><div><p>{t.masthead}</p><h1>{t.featuredNews}</h1></div><LanguagePicker locale={locale} setLocale={setLocale} /></header>;
});
const goldPrice = (value: number) => new Intl.NumberFormat("vi-VN", { style: "currency", currency: "VND", maximumFractionDigits: 0 }).format(value);
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
  const highestSell = Math.max(...rates.map(rate => rate.sell_price), 1);
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
                    {goldPrice(Number(rate.trend_value.replace(/[^\d.-]/g, "")))}
                  </p>
                )}
              </article>
            ))}
          </div>
          <section className="gold-chart" aria-labelledby="gold-chart-title">
            <div className="gold-chart-heading"><strong id="gold-chart-title">So sánh giá bán</strong><span>VND/lượng</span></div>
            <div className="gold-chart-bars" role="img" aria-label={locale === "vi" ? "Biểu đồ so sánh giá bán vàng theo loại" : "Gold sell-price comparison chart"}>
              {rates.map(rate => <div className="gold-chart-bar" key={rate.code}><div className="gold-chart-plot"><span style={{ height: `${Math.max(8, rate.sell_price / highestSell * 100)}%` }} /></div><strong>{goldPrice(rate.sell_price)}</strong><small>{rate.code}</small></div>)}
            </div>
          </section>
          <p className="gold-rates-meta">
            {updated && <span>{t.goldUpdated}: {goldTime(updated)}</span>}
            <span>{t.goldDisclaimer}</span>
          </p>
        </>
      )}
    </section>
  );
}
function ArticleCategories({ article, locale }: { article: Article; locale: Locale }) {
  const labels = article.categories?.length ? article.categories : [{ slug: article.category, name: article.category }];
  return <span className="category-labels" aria-label={locale === "vi" ? "Thể loại" : "Categories"}>{labels.map(item => <span className={`category-label category-${item.slug}`} key={item.slug}>{categories[item.slug]?.[locale] ?? item.name}</span>)}</span>;
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
          <ArticleCategories article={article} locale={locale} />
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
      <button
        type="button"
        className="share"
        aria-label={`${t.share}: ${article.title}`}
        onClick={() => void shareArticle(article)}
      >
        <Icon name="share" />
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
  const [readingMode, setReadingMode] = useState(false);
  const t = text[locale];
  const resummarize = async () => {
    setResummarizing(true);
    setResummarizeFailed(false);
    try {
      const brief = await api.resummarize(article.id);
      setManualBrief(brief);
    } catch {
      setResummarizeFailed(true);
    } finally {
      setResummarizing(false);
    }
  };
  useEffect(() => {
    setManualBrief(null);
    setResummarizeFailed(false);
    setIsRead(article.is_read);
    setFeedbackIssue(null);
    setFeedbackReason("");
    setFeedbackSent(false);
    setFeedbackFailed(false);
    setReadingMode(false);
  }, [article.id]);
  useEffect(() => {
    void api.startReading(article.id);
  }, [article.id]);
  const title = manualBrief?.title || article.title;
  const summary = manualBrief?.summary || article.summary;
  const summaryReadingTime = t.summaryReadingTime.replace(
    "{minutes}",
    String(estimateReadingMinutes(summary)),
  );
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
    <section className={`page detail ${readingMode ? "reading-mode" : ""}`}>
      <button className="back-button" onClick={back}>
        <Icon name="arrow-left" />
        {t.back}
      </button>
      <p className="eyebrow">
        {article.source} · {publishedOn(article.published_at, locale)}
      </p>
      <ArticleCategories article={article} locale={locale} />
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
      <div className="reading-tools" aria-label={locale === "vi" ? "Công cụ đọc bài" : "Reading tools"}>
        <span className="reading-time" aria-label={summaryReadingTime}>{summaryReadingTime}</span>
        <button
          type="button"
          className="read-mode-button"
          aria-pressed={readingMode}
          onClick={() => setReadingMode((enabled) => !enabled)}
        >
          {readingMode ? t.exitReadMode : t.readMode}
        </button>
        <button
          type="button"
          className="read-mode-button"
          onClick={() => void shareArticle(article)}
        >
          <Icon name="share" />
          {t.share}
        </button>
      </div>
      {article.image_url && (
        <img className="detail-image" src={article.image_url} alt="" />
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
      <section className="article-content original-content">
        <small>{t.originalContent}</small>
        <RSSDescription
          description={article.original_content || article.description}
        />
      </section>
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
  const toggleHideRead = useCallback(() => setHideRead(value => !value), []);
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
      <HomeMasthead saved={saved} history={history} locale={locale} setLocale={setLocale} theme={theme} toggleTheme={toggleTheme} hideRead={hideRead} toggleHideRead={toggleHideRead} />
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
        <FilterSelect id="saved-sort" className="saved-sort-select" label={t.sortSaved} icon="sort" value={savedSort} onChange={setSavedSort} options={[{ value: "saved_newest", label: t.savedNewest }, { value: "saved_oldest", label: t.savedOldest }, { value: "published_newest", label: t.publishedNewest }, { value: "title", label: t.titleSort }]} />
        <FilterSelect id="saved-folder" label={t.folders} icon="folder" value={savedFolder} onChange={setSavedFolder} options={[{ value: "", label: t.all }, ...savedOrganization.folders.map(folder => ({ value: String(folder.id), label: `${folder.name} (${folder.count})` }))]} />
        <FilterSelect id="saved-tag" label={t.tags} icon="tag" value={savedTag} onChange={setSavedTag} options={[{ value: "", label: t.all }, ...savedOrganization.tags.map(tag => ({ value: String(tag.id), label: `${tag.name} (${tag.count})` }))]} />
        <button className="text-button" onClick={() => void addCollection("folder")}>{t.addFolder}</button><button className="text-button" onClick={() => void addCollection("tag")}>{t.addTag}</button>
        {selectedIDs.length > 0 && <><button className="text-button" onClick={() => void removeSelected()}>{t.removeSelected} ({selectedIDs.length})</button><select className="compact-assignment-select" defaultValue="" onChange={event => { void assignCollection("folder", Number(event.target.value)); event.currentTarget.value = ""; }}><option value="">{t.folders}</option>{savedOrganization.folders.map(folder => <option key={folder.id} value={folder.id}>{folder.name}</option>)}</select><select className="compact-assignment-select" defaultValue="" onChange={event => { void assignCollection("tag", Number(event.target.value)); event.currentTarget.value = ""; }}><option value="">{t.tags}</option>{savedOrganization.tags.map(tag => <option key={tag.id} value={tag.id}>{tag.name}</option>)}</select></>}
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
      <FeaturedMasthead locale={locale} setLocale={setLocale} />
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
              {t.featuredCoverage} · {t.featuredUpdated} {publishedOn(brief.generated_at, locale)}
            </p>
            <h2>{brief.title}</h2>
            <p>{brief.intro}</p>
          </section>
          {brief.takeaways.length > 0 && (
            <section className="featured-takeaways" aria-labelledby="featured-takeaways-title">
              <p className="section-kicker" id="featured-takeaways-title">{t.featuredTakeaways}</p>
              <ol>
                {brief.takeaways.map((takeaway, index) => <li key={`${index}-${takeaway}`}>{takeaway}</li>)}
              </ol>
            </section>
          )}
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
  const t = text[locale];
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
      {topic.why_it_matters && <aside className="featured-why" aria-label={t.featuredWhyItMatters}><strong>{t.featuredWhyItMatters}</strong><span>{topic.why_it_matters}</span></aside>}
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
  openReader,
}: {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  openReader: () => void;
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
      <div className="settings-card reader-settings-card">
        <small>TOOLS</small>
        <h2>{t.mediumReader}</h2>
        <p>{t.mediumReaderHint}</p>
        <button className="text-button" onClick={openReader}>{t.mediumOpen}</button>
      </div>
    </section>
  );
}
function MediumReader({ locale, back }: { locale: Locale; back: () => void }) {
  const t = text[locale];
  const [url, setURL] = useState("");
  const [article, setArticle] = useState<MediumReaderArticle | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!url.trim()) return;
    setLoading(true); setError(""); setArticle(null);
    try { setArticle(await api.readMedium(url.trim())); }
    catch (caught) { setError(caught instanceof Error ? caught.message : t.loadError); }
    finally { setLoading(false); }
  };
  const accessLabel = article?.access === "author_free_link" ? t.mediumFree : article?.access === "preview" ? t.mediumPreview : t.mediumPublic;
  return <section className="page reader-page">
    <button className="back-button" onClick={back}><Icon name="arrow-left" />{t.back}</button>
    <p className="section-kicker">TOOLS</p>
    <h1>{t.mediumReader}</h1>
    <p className="reader-intro">{t.mediumReaderHint}</p>
    <form className="reader-form" onSubmit={(event) => void submit(event)} aria-busy={loading}>
      <label htmlFor="medium-url">{t.mediumURL}</label>
      <div className="reader-form-row">
        <input id="medium-url" type="url" inputMode="url" autoComplete="url" placeholder="https://medium.com/@author/story" value={url} onChange={(event) => setURL(event.target.value)} required aria-describedby="medium-help medium-error" />
        <button className="primary" disabled={loading || !url.trim()}>{loading ? t.mediumLoading : t.mediumOpen}</button>
      </div>
      <p id="medium-help" className="reader-help">medium.com hoặc một subdomain Medium; chỉ nội dung công khai.</p>
      {error && <p id="medium-error" className="reader-error" role="alert">{error}</p>}
    </form>
    {article && <article className="reader-article">
      <div className={`reader-access reader-access-${article.access}`} role={article.access === "preview" ? "alert" : "status"}>{accessLabel}</div>
      <h2>{article.title}</h2>
      {article.subtitle && <p className="reader-subtitle">{article.subtitle}</p>}
      {(article.author || article.published_at || article.reading_time_minutes) && <p className="reader-meta">{[article.author, article.published_at && publishedOn(article.published_at, locale), article.reading_time_minutes && `${article.reading_time_minutes} min`].filter(Boolean).join(" · ")}</p>}
      {(article.warning || article.access === "preview") && <p className="reader-warning">{article.warning || t.mediumPreviewWarning}</p>}
      <div className="reader-content" dangerouslySetInnerHTML={{ __html: article.content_html }} />
      <button className="primary reader-original" onClick={() => open(article.resolved_url || article.canonical_url)}>{t.mediumOriginal}<Icon name="arrow-up-right" /></button>
    </article>}
  </section>;
}
function ScrollToTop({ locale, compact }: { locale: Locale; compact: boolean }) {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    const updateVisibility = () => setVisible(window.scrollY > 360);
    updateVisibility();
    window.addEventListener("scroll", updateVisibility, { passive: true });
    return () => window.removeEventListener("scroll", updateVisibility);
  }, []);
  if (!visible) return null;
  return (
    <button
      type="button"
      className={`scroll-to-top ${compact ? "scroll-to-top-content" : ""}`}
      aria-label={text[locale].scrollToTop}
      title={text[locale].scrollToTop}
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
    >
      <Icon name="arrow-up" />
    </button>
  );
}
const BottomNavigation = memo(function BottomNavigation({
  tab,
  locale,
  onSelect,
}: {
  tab: Tab;
  locale: Locale;
  onSelect: (tab: Tab) => void;
}) {
  const t = text[locale];
  const navItems: [Tab, Parameters<typeof Icon>[0]["name"], string][] = [
    ["home", "home", t.home],
    ["featured", "sparkles", t.featuredNews],
    ["saved", "bookmark", t.saved],
    ["history", "history", t.history],
    ["settings", "settings", t.settings],
  ];
  return (
    <nav aria-label="Primary navigation">
      {navItems.map(([id, icon, label]) => (
        <button
          className={tab === id ? "nav-active" : ""}
          aria-current={tab === id ? "page" : undefined}
          aria-label={label}
          onClick={() => onSelect(id)}
          key={id}
        >
          <Icon name={icon} filled={id === "saved" && tab === id} />
          <span className="nav-label">{label}</span>
        </button>
      ))}
    </nav>
  );
});
type AdminStatus = Awaited<ReturnType<typeof api.adminStatus>>;
type AdminSource = AdminStatus["sources"][number];

function formatAdminTime(value: string) {
  if (!value) return "Chưa có";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("vi-VN", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
function formatJobDuration(startedAt: string, finishedAt = "") {
  const started = new Date(startedAt).getTime();
  if (Number.isNaN(started)) return "";
  const seconds = Math.floor(Math.max(0, new Date(finishedAt || Date.now()).getTime() - started) / 1000);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remainingSeconds = seconds % 60;
  const parts = [];
  if (hours) parts.push(`${hours} giờ`);
  if (minutes || hours) parts.push(`${minutes} phút`);
  parts.push(`${remainingSeconds} giây`);
  return parts.join(" ");
}

function AdminSources({
  sources,
  token,
  action,
}: {
  sources: AdminSource[];
  token: string;
  action: (work: () => Promise<unknown>) => Promise<boolean>;
}) {
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<"all" | "enabled" | "issues" | "disabled">("all");
  const [updatingID, setUpdatingID] = useState<number | null>(null);
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const [operation, setOperation] = useState("");
  const [operationMessage, setOperationMessage] = useState("");
  const counts = {
    all: sources.length,
    enabled: sources.filter(source => source.enabled).length,
    issues: sources.filter(source => source.enabled && source.last_error).length,
    disabled: sources.filter(source => !source.enabled).length,
  };
  const normalizedQuery = query.trim().toLocaleLowerCase("vi");
  const visibleSources = sources.filter(source => {
    const matchesQuery = !normalizedQuery || source.name.toLocaleLowerCase("vi").includes(normalizedQuery);
    const matchesFilter =
      filter === "all" ||
      (filter === "enabled" && source.enabled) ||
      (filter === "issues" && source.enabled && Boolean(source.last_error)) ||
      (filter === "disabled" && !source.enabled);
    return matchesQuery && matchesFilter;
  });
  const toggleSource = async (source: AdminSource) => {
    setUpdatingID(source.id);
    if (source.enabled) setSelectedIDs(current => current.filter(id => id !== source.id));
    try {
      await action(() => api.adminUpdateSource(token, source.id, !source.enabled));
    } finally {
      setUpdatingID(null);
    }
  };
  const toggleSelected = (sourceID: number) => {
    setSelectedIDs(current => current.includes(sourceID) ? current.filter(id => id !== sourceID) : [...current, sourceID]);
  };
  const selectableIDs = visibleSources.filter(source => source.enabled).map(source => source.id);
  const allVisibleSelected = selectableIDs.length > 0 && selectableIDs.every(id => selectedIDs.includes(id));
  const toggleAllVisible = () => {
    setSelectedIDs(current => allVisibleSelected
      ? current.filter(id => !selectableIDs.includes(id))
      : [...new Set([...current, ...selectableIDs])]);
  };
  const runFetch = async (sourceIDs: number[], key: string) => {
    setOperation(key);
    setOperationMessage("");
    const succeeded = await action(() => api.adminFetchRSS(token, sourceIDs));
    if (succeeded) setOperationMessage(sourceIDs.length ? `Đã bắt đầu chạy ${sourceIDs.length} nguồn.` : "Đã bắt đầu chạy tất cả nguồn đang bật.");
    setOperation("");
  };
  const runTranslation = async () => {
    setOperation("translate");
    setOperationMessage("");
    let queued = 0;
    const succeeded = await action(async () => {
      const response = await api.adminEnqueueTranslations(token, selectedIDs);
      queued = response.queued;
      return response;
    });
    if (succeeded) setOperationMessage(`Đã xếp ${queued} bài vào hàng đợi dịch từ ${selectedIDs.length} nguồn.`);
    setOperation("");
  };
  const filters = [
    { id: "all" as const, label: "Tất cả", value: counts.all },
    { id: "enabled" as const, label: "Đang bật", value: counts.enabled },
    { id: "issues" as const, label: "Có lỗi", value: counts.issues },
    { id: "disabled" as const, label: "Đã tắt", value: counts.disabled },
  ];
  return (
    <section className="settings-card admin-sources" aria-labelledby="admin-sources-title">
      <div className="source-section-heading">
        <div>
          <small>HỆ THỐNG PHÂN PHỐI</small>
          <h2 id="admin-sources-title">Nguồn tin</h2>
          <p>Theo dõi trạng thái crawler và kiểm soát nguồn đang xuất hiện trong feed.</p>
        </div>
        <span className="source-total">{counts.enabled}/{counts.all} hoạt động</span>
      </div>
      <div className="source-stats" aria-label="Lọc nguồn theo trạng thái">
        {filters.map(item => (
          <button
            type="button"
            className={filter === item.id ? `source-stat ${item.id} selected` : `source-stat ${item.id}`}
            aria-pressed={filter === item.id}
            onClick={() => setFilter(item.id)}
            key={item.id}
          >
            <strong>{item.value}</strong>
            <span>{item.label}</span>
          </button>
        ))}
      </div>
      <label className="source-admin-search" htmlFor="admin-source-search">
        <span className="sr-only">Tìm nguồn tin</span>
        <FilterIcon name="search" />
        <input
          id="admin-source-search"
          type="search"
          value={query}
          onChange={event => setQuery(event.target.value)}
          placeholder="Tìm theo tên nguồn…"
          autoComplete="off"
        />
        <span className="source-result-count" aria-live="polite">{visibleSources.length} kết quả</span>
      </label>
      <div className="source-bulk-bar">
        <label className="source-select-all">
          <input type="checkbox" checked={allVisibleSelected} onChange={toggleAllVisible} />
          <span>Chọn nguồn đang hiển thị</span>
        </label>
        <div className="source-bulk-actions">
          <button type="button" className="secondary-button" disabled={Boolean(operation)} onClick={() => void runFetch([], "all")}>{operation === "all" ? "Đang chạy…" : "Chạy tất cả"}</button>
          <button type="button" className="secondary-button" disabled={Boolean(operation) || !selectedIDs.length} onClick={() => void runFetch(selectedIDs, "selected")}>{operation === "selected" ? "Đang chạy…" : `Chạy đã chọn (${selectedIDs.length})`}</button>
          <button type="button" className="primary" disabled={Boolean(operation) || !selectedIDs.length} onClick={() => void runTranslation()}>{operation === "translate" ? "Đang xếp hàng…" : `Dịch hàng loạt (${selectedIDs.length})`}</button>
        </div>
      </div>
      {operationMessage && <p className="source-operation-message" role="status">{operationMessage}</p>}
      <div className="admin-source-list">
        {visibleSources.map(source => {
          const health = !source.enabled ? "disabled" : source.last_error ? "issue" : source.last_success_at ? "healthy" : "pending";
          const healthLabel = health === "healthy" ? "Ổn định" : health === "issue" ? "Có lỗi" : health === "disabled" ? "Đã tắt" : "Chờ dữ liệu";
          return (
            <article className={`admin-source-card ${health}${selectedIDs.includes(source.id) ? " selected" : ""}`} key={source.id}>
              <div className="source-card-heading">
                <div className="source-identity">
                  <label className="source-checkbox">
                    <input type="checkbox" checked={selectedIDs.includes(source.id)} disabled={!source.enabled} onChange={() => toggleSelected(source.id)} />
                    <span className="sr-only">Chọn nguồn {source.name}</span>
                  </label>
                  <span className="source-health-dot" aria-hidden="true" />
                  <div>
                    <h3>{source.name}</h3>
                    <span className={`source-health-label ${health}`}>{healthLabel}</span>
                  </div>
                </div>
                <div className="source-row-actions">
                  <button type="button" className="source-run-button" aria-label={`Chạy nguồn ${source.name}`} title={`Chạy nguồn ${source.name}`} disabled={!source.enabled || Boolean(operation)} onClick={() => void runFetch([source.id], `source-${source.id}`)}>{operation === `source-${source.id}` ? <span className="loading-spinner" aria-hidden="true" /> : <Icon name="play" />}</button>
                  <button
                    type="button"
                    className={source.enabled ? "source-toggle enabled" : "source-toggle"}
                    disabled={updatingID === source.id}
                    aria-label={`${source.enabled ? "Tắt" : "Bật"} nguồn ${source.name}`}
                    onClick={() => void toggleSource(source)}
                  >
                    <Icon name="power" />
                  </button>
                </div>
              </div>
              <dl className="source-metrics">
                <div><dt>Thành công gần nhất</dt><dd>{formatAdminTime(source.last_success_at)}</dd></div>
                <div><dt>Bài mới</dt><dd>{source.last_inserted}</dd></div>
              </dl>
              {source.last_error && (
                <div className="source-error" role="status">
                  <strong>Lỗi gần nhất</strong>
                  <p>{source.last_error}</p>
                </div>
              )}
            </article>
          );
        })}
        {!visibleSources.length && (
          <div className="source-empty">
            <strong>Không tìm thấy nguồn phù hợp</strong>
            <p>Thử đổi từ khoá hoặc chọn trạng thái khác.</p>
          </div>
        )}
      </div>
    </section>
  );
}

function AdminPage() {
  const [token, setToken] = useState(() => sessionStorage.getItem("admin-token") || "");
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [aiForm, setAIForm] = useState<AIConfigInput>({ base_url: "", api_key: "", model: "" });
  const [apiKeyConfigured, setAPIKeyConfigured] = useState(false);
  const [configSource, setConfigSource] = useState<"env" | "saved">("env");
  const [models, setModels] = useState<AIModel[]>([]);
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [testReply, setTestReply] = useState("");
  const [testError, setTestError] = useState("");
  const [featuredArticleCount, setFeaturedArticleCount] = useState(8);
  const [jobsPage, setJobsPage] = useState(1);
  const [jobsTab, setJobsTab] = useState<"running" | "completed" | "failed">("running");
  const load = async (page = jobsPage) => {
    setBusy("load");
    try {
      const [next, aiConfig] = await Promise.all([api.adminStatus(token, page), api.adminAIConfig(token)]);
      sessionStorage.setItem("admin-token", token);
      setStatus(next);
      setAIForm({ base_url: aiConfig.base_url, api_key: "", model: aiConfig.model });
      setAPIKeyConfigured(aiConfig.api_key_configured);
      setConfigSource(aiConfig.source);
      setError("");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Không thể tải dữ liệu hoặc token quản trị không hợp lệ.");
    } finally {
      setBusy("");
    }
  };
  const action = async (work: () => Promise<unknown>) => {
    try {
      await work();
      await load();
      return true;
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Thao tác không thành công.");
      return false;
    }
  };
  const cancelJob = async (id: number) => {
    setBusy(`cancel-${id}`); setError("");
    try {
      await api.adminCancelJob(token, id);
      setMessage("Đã gửi yêu cầu hủy tác vụ.");
      await load();
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Không thể hủy tác vụ."); }
    finally { setBusy(""); }
  };
  const clearJobHistory = async () => {
    if (jobsTab === "running") return;
    setBusy("clear-jobs"); setError("");
    try {
      const result = await api.adminClearJobHistory(token, jobsTab);
      setMessage(`Đã xóa ${result.cleared} tác vụ trong tab hiện tại.`);
      setJobsPage(1);
      await load(1);
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Không thể xóa lịch sử tác vụ."); }
    finally { setBusy(""); }
  };
  const loadModels = async () => {
    setBusy("models"); setError(""); setMessage("");
    try {
      const next = await api.adminAIModels(token, aiForm);
      setModels(next);
      setAIForm(current => ({
        ...current,
        model: next.some(model => model.id === current.model)
          ? current.model
          : next[0]?.id || current.model,
      }));
      setMessage(`Đã tải ${next.length} model từ /v1/models.`);
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Không thể tải danh sách model."); }
    finally { setBusy(""); }
  };
  const testModel = async () => {
    setBusy("test"); setError(""); setMessage(""); setTestReply(""); setTestError("");
    try {
      const result = await api.adminAITest(token, aiForm);
      setTestReply(result.reply || "OK");
    } catch (caught) { setTestError(caught instanceof Error ? caught.message : "Model không hoạt động."); }
    finally { setBusy(""); }
  };
  const saveAI = async () => {
    setBusy("save"); setError(""); setMessage("");
    try {
      const saved = await api.adminAISave(token, aiForm);
      setAIForm({ base_url: saved.base_url, api_key: "", model: saved.model });
      setAPIKeyConfigured(saved.api_key_configured); setConfigSource(saved.source);
      setMessage("Đã lưu cấu hình AI và áp dụng ngay.");
      await load();
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Không thể lưu cấu hình AI."); }
    finally { setBusy(""); }
  };
  const resetAI = async () => {
    setBusy("reset"); setError(""); setMessage("");
    try {
      const saved = await api.adminAIReset(token);
      setAIForm({ base_url: saved.base_url, api_key: "", model: saved.model });
      setAPIKeyConfigured(saved.api_key_configured); setConfigSource(saved.source); setModels([]);
      setMessage("Đã xoá cấu hình đã lưu và quay lại giá trị từ env.");
      await load();
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Không thể khôi phục cấu hình env."); }
    finally { setBusy(""); }
  };
  const generateDailyBrief = async () => {
    const articleCount = Math.min(12, Math.max(3, featuredArticleCount || 8));
    setFeaturedArticleCount(articleCount);
    setBusy("featured"); setError(""); setMessage("");
    try {
      await api.adminRegenerateFeatured(token, articleCount);
      setMessage(`Đã bắt đầu tạo Bản tin hằng ngày với ${articleCount} tin. Quá trình này chạy nền; tải lại dashboard sau ít phút để xem kết quả.`);
      window.setTimeout(() => {
        void api.adminStatus(token).then(setStatus).catch(() => {});
      }, 300);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Không thể tạo bản tin hằng ngày.");
    } finally { setBusy(""); }
  };
  useEffect(() => {
    if (!status || !token || (status.translation_queue === 0 && !status.jobs.some(job => job.status === "running"))) return;
    const refresh = window.setInterval(() => {
      void api.adminStatus(token, jobsPage).then(setStatus).catch(() => {});
    }, 5_000);
    return () => window.clearInterval(refresh);
  }, [status, token, jobsPage]);
  return (
    <main className="admin-shell">
      <div className="app-shell">
        <section className="page settings admin-page">
          <header className="admin-hero">
            <div>
              <p className="section-kicker">SIGNAL BRIEF / ADMIN</p>
              <h1>Trung tâm vận hành</h1>
              <p>Quản lý AI, tạo bản tin hằng ngày và theo dõi nguồn tin trong một không gian tập trung.</p>
            </div>
            {status && <span className={`admin-readiness ${status.ai.configured ? "ready" : ""}`}>{status.ai.configured ? "AI sẵn sàng" : "Cần cấu hình AI"}</span>}
          </header>
          <section className="admin-access-card" aria-labelledby="admin-access-title">
            <div>
              <small>QUYỀN QUẢN TRỊ</small>
              <h2 id="admin-access-title">Mở dashboard</h2>
              <p>Nhập admin token để tải dữ liệu vận hành mới nhất.</p>
            </div>
            <form className="admin-login" onSubmit={event => { event.preventDefault(); void load(); }}>
              <label className="admin-field">
                <span>ADMIN TOKEN</span>
                <input type="password" value={token} onChange={event => setToken(event.target.value)} autoComplete="current-password" />
              </label>
              <button className="primary" disabled={!token || busy === "load"}>
                {busy === "load" ? "Đang tải…" : status ? "Làm mới dashboard" : "Tải dashboard"}
              </button>
            </form>
          </section>
          {error && <p className="admin-notice error" role="alert">{error}</p>}
          {message && <p className="admin-notice success" role="status" aria-live="polite">{message}</p>}
          {status && (
            <div className="admin-workspace">
              <section className="settings-card ai-config-card" aria-labelledby="ai-config-title">
                <div className="admin-panel-heading">
                  <div>
                    <small>CẤU HÌNH AI</small>
                    <h2 id="ai-config-title">Kết nối nhà cung cấp AI</h2>
                    <p>Chọn endpoint, xác thực và model dùng để dịch và tạo bản tin nổi bật.</p>
                  </div>
                  <div className="config-statuses" aria-label="Trạng thái cấu hình AI">
                    <span className="config-badge">{configSource === "env" ? "Theo ENV" : "Đã lưu"}</span>
                    <span className={`config-badge ${apiKeyConfigured ? "configured" : ""}`}>{apiKeyConfigured ? "API key sẵn sàng" : "Thiếu API key"}</span>
                  </div>
                </div>
                <form className="ai-config-form" onSubmit={event => { event.preventDefault(); void saveAI(); }}>
                  <div className="ai-form-grid">
                    <label className="admin-field ai-base-url"><span>AI BASE URL</span><input type="url" value={aiForm.base_url} onChange={event => setAIForm(current => ({ ...current, base_url: event.target.value }))} placeholder="https://api.openai.com" /></label>
                    <label className="admin-field"><span>API KEY</span><input type="password" value={aiForm.api_key} onChange={event => setAIForm(current => ({ ...current, api_key: event.target.value }))} placeholder={apiKeyConfigured ? "Để trống để giữ key hiện tại" : "Nhập API key"} autoComplete="new-password" /></label>
                    <div className="model-row">
                      <label className="admin-field">
                        <span>MODEL</span>
                        <select value={aiForm.model} onChange={event => setAIForm(current => ({ ...current, model: event.target.value }))} aria-describedby="ai-model-help">
                          <option value="" disabled>Bấm “Lấy models” để tải danh sách</option>
                          {aiForm.model && !models.some(model => model.id === aiForm.model) && <option value={aiForm.model}>{aiForm.model} (hiện tại)</option>}
                          {models.map(model => <option value={model.id} key={model.id}>{model.id}{model.owned_by ? ` · ${model.owned_by}` : ""}</option>)}
                        </select>
                      </label>
                      <button type="button" className="secondary-button" disabled={busy !== "" || !aiForm.base_url} onClick={() => void loadModels()}>{busy === "models" ? "Đang lấy…" : "Lấy models"}</button>
                    </div>
                  </div>
                  <p className="admin-hint" id="ai-model-help">{models.length > 0 ? `Đã tìm thấy ${models.length} model. Chọn một model rồi kiểm tra kết nối trước khi lưu.` : "Danh sách model được lấy trực tiếp từ endpoint /v1/models của provider."}</p>
                  <div className="admin-actions">
                    <button type="button" className="secondary-button" disabled={busy !== "" || !aiForm.model} onClick={() => void testModel()}>{busy === "test" ? "Đang kiểm tra…" : "Test model"}</button>
                    <button type="submit" className="primary" disabled={busy !== "" || !aiForm.base_url || !aiForm.model}>{busy === "save" ? "Đang lưu…" : "Lưu cấu hình"}</button>
                    <button type="button" className="text-button" disabled={busy !== "" || configSource === "env"} onClick={() => void resetAI()}>Khôi phục ENV</button>
                  </div>
                  {testReply && (
                    <section className="ai-test-result success" role="status" aria-live="polite">
                      <strong>Model hoạt động</strong>
                      <pre>{testReply}</pre>
                    </section>
                  )}
                  {testError && (
                    <section className="ai-test-result error" role="alert">
                      <strong>Không thể test model</strong>
                      <p>{testError}</p>
                    </section>
                  )}
                </form>
              </section>
              <div className="admin-dashboard-grid">
                <section className="settings-card featured-generator-card" aria-labelledby="featured-generator-title">
                <small>BẢN TIN HẰNG NGÀY</small>
                <h2 id="featured-generator-title">Tạo bản tin theo yêu cầu</h2>
                <p>Chọn số lượng tin trước khi tạo. Bản tin được tạo nền từ các bài mới nhất trong 24 giờ qua.</p>
                <div className="featured-generator-controls">
                  <label className="featured-count-field">
                    <span>Số tin</span>
                    <input type="number" min="3" max="12" value={featuredArticleCount} onChange={event => setFeaturedArticleCount(Number(event.target.value))} disabled={busy !== "" || !status.ai.configured} />
                  </label>
                  <button
                    type="button"
                    className="primary"
                    disabled={busy !== "" || !status.ai.configured}
                    onClick={() => void generateDailyBrief()}
                  >
                    {busy === "featured" ? "Đang bắt đầu tạo…" : "Tạo bản tin"}
                  </button>
                </div>
                {!status.ai.configured && <p className="featured-generator-note" role="status">Cần hoàn tất cấu hình AI trước khi tạo bản tin.</p>}
                </section>
                <section className="settings-card operations-card" aria-labelledby="operations-title">
                <div className="admin-panel-heading operations-heading">
                  <div>
                    <small>RSS & AI</small>
                    <h2 id="operations-title">Tình trạng vận hành</h2>
                    <p>Theo dõi hàng đợi xử lý và đầu ra của hệ thống tự động.</p>
                  </div>
                  <span className={`config-badge ${status.ai.configured ? "configured" : ""}`}>{status.ai.configured ? "AI đang sẵn sàng" : "AI chưa sẵn sàng"}</span>
                </div>
                <dl className="operations-metrics">
                  <div><dt>Hàng đợi dịch</dt><dd>{status.translation_queue}</dd><small>Bài đang chờ xử lý</small></div>
                  <div><dt>Bản dịch</dt><dd>{status.ai.translations_generated}</dd><small>Đã tạo</small></div>
                  <div><dt>Featured brief</dt><dd>{status.ai.featured_briefs}</dd><small>Đã tạo</small></div>
                  <div><dt>Phản hồi</dt><dd>{status.ai.feedback}</dd><small>Từ độc giả</small></div>
                </dl>
                <div className="operations-footer">
                  <p><strong>Model đang dùng</strong><span>{status.ai.model || "Chưa chọn model"}</span></p>
                  <p className="operations-note">{status.ai.cost_tracking}</p>
                </div>
                </section>
              </div>
              <section className="settings-card admin-jobs" aria-labelledby="admin-jobs-title">
                  <div className="admin-jobs-heading">
                    <div>
                      <small>JOBS</small>
                      <h3 id="admin-jobs-title">Tiến trình tác vụ</h3>
                    </div>
                    <div className="jobs-header-actions"><span className="jobs-running-count">{status.jobs.filter(job => job.status === "running" || job.status === "queued").length} đang chạy</span>{jobsTab !== "running" && <button type="button" className="text-button" disabled={busy === "clear-jobs" || !status.jobs.some(job => job.status === jobsTab)} onClick={() => void clearJobHistory()}>{busy === "clear-jobs" ? "Đang xóa…" : "Xóa tab này"}</button>}</div>
                  </div>
                  <div className="jobs-tabs" role="tablist" aria-label="Lọc tác vụ">
                    {(["running", "completed", "failed"] as const).map(tab => <button type="button" role="tab" aria-selected={jobsTab === tab} className={jobsTab === tab ? "active" : ""} onClick={() => setJobsTab(tab)} key={tab}>{tab === "running" ? "Đang chạy" : tab === "completed" ? "Thành công" : "Thất bại"}</button>)}
                  </div>
                  {(() => { const visibleJobs = status.jobs.filter(job => jobsTab === "running" ? job.status === "running" || job.status === "queued" : jobsTab === "completed" ? job.status === "completed" : job.status === "failed"); return visibleJobs.length ? (
                    <ul className="admin-job-list">
                      {visibleJobs.map(job => (
                        <li key={`${job.kind}-${job.id}-${job.started_at}`}>
                          <span className={`job-status job-status-${job.status}`}>{job.status === "running" ? "Đang chạy" : job.status === "queued" ? "Đang chờ" : job.status === "completed" ? "Hoàn tất" : job.status === "skipped" ? "Đã bỏ qua" : "Thất bại"}</span>
                          <div>
                            <strong>{job.title}</strong>
                            <p>{job.status === "running" ? (job.stage || "Đang chuẩn bị tác vụ") : (job.detail || job.stage || "Chưa có chi tiết")}</p>
                            {job.target_count > 0 && <><div className="job-progress" aria-label={`Tiến độ ${job.completed_count}/${job.target_count}`}><span style={{ width: `${Math.min(100, job.completed_count / job.target_count * 100)}%` }} /></div><div className="job-meta"><span>{job.completed_count}/{job.target_count} hoàn tất</span>{job.failed_count > 0 && <span>{job.failed_count} lỗi</span>}<span>{job.trigger === "scheduled" ? "Tự động" : "Thủ công"}</span></div></>}
                            {job.detail && job.status === "running" && <small className="job-detail">{job.detail}</small>}
                          </div>
                          <div className="job-side">{job.status === "running" && <button type="button" className="secondary-button job-cancel" disabled={busy === `cancel-${job.id}`} onClick={() => void cancelJob(job.id)}>{busy === `cancel-${job.id}` ? "Đang hủy…" : "Hủy"}</button>}{job.started_at && <time dateTime={job.started_at}>Bắt đầu {formatAdminTime(job.started_at)}<br />{job.finished_at ? `Mất ${formatJobDuration(job.started_at, job.finished_at)}` : `Đã chạy ${formatJobDuration(job.started_at)}`}</time>}</div>
                        </li>
                      ))}
                    </ul>
                  ) : <p className="admin-jobs-empty">Không có tác vụ trong mục này.</p>; })()}
                  {status.jobs_total > status.jobs_page_size && <div className="jobs-pagination"><button type="button" className="secondary-button" disabled={status.jobs_page <= 1} onClick={() => { const page = status.jobs_page - 1; setJobsPage(page); void load(page); }}>Trước</button><span>Trang {status.jobs_page}/{Math.ceil(status.jobs_total / status.jobs_page_size)}</span><button type="button" className="secondary-button" disabled={status.jobs_page >= Math.ceil(status.jobs_total / status.jobs_page_size)} onClick={() => { const page = status.jobs_page + 1; setJobsPage(page); void load(page); }}>Sau</button></div>}
              </section>
              <section className="settings-card usage-card" aria-labelledby="usage-title">
                <small>AI USAGE</small><h2 id="usage-title">Token theo ngày</h2>
                {status.ai_usage.length ? (() => { const max = Math.max(...status.ai_usage.map(item => item.total_tokens)); return <div className="usage-chart" aria-label="Biểu đồ token AI theo ngày">{status.ai_usage.map(item => <div className="usage-bar" key={item.day}><span style={{ height: `${Math.max(8, item.total_tokens / max * 100)}%` }} /><strong>{item.total_tokens.toLocaleString()}</strong><small>{item.day.slice(5)}</small></div>)}</div>; })() : <p className="admin-jobs-empty">Chưa có dữ liệu token. Dữ liệu được ghi nhận từ yêu cầu AI tiếp theo.</p>}
              </section>
              <AdminSources sources={status.sources} token={token} action={action} />
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
export default function App() {
  if (window.location.pathname === "/admin") return <AdminPage />;
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
  const [route, setRoute] = useState(() => window.location.pathname);
  const [tab, setTab] = useState<Tab>("home");
  const [article, setArticle] = useState<Article | null>(null);
  const [articleID, setArticleID] = useState<number | null>(articleIDFromPath);
  const [detailError, setDetailError] = useState(false);
  const [detailReload, setDetailReload] = useState(0);
  const listingScroll = useRef(0);
  const setLocale = useCallback((next: Locale) => {
    localStorage.setItem("locale", next);
    setLocaleState(next);
  }, []);
  const t = text[locale];
  const theme = themeOverride ?? automaticTheme;
  const toggleTheme = useCallback(() => {
    const next = theme === "dark" ? "light" : "dark";
    localStorage.setItem("theme", next);
    setThemeOverride(next);
  }, [theme]);
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
    const onPopState = () => { setArticleID(articleIDFromPath()); setRoute(window.location.pathname); };
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
  const openReader = () => {
    window.history.pushState({}, "", "/reader");
    setRoute("/reader");
    window.scrollTo(0, 0);
  };
  const backFromReader = () => {
    if (window.history.state !== null) window.history.back();
    else { window.history.replaceState({}, "", "/"); setRoute("/"); }
  };
  const showingDetail = articleID !== null;
  const showingReader = route === "/reader";
  return (
    <main>
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <div className="app-shell" id="main-content" tabIndex={-1}>
        <div
          className={showingDetail || showingReader ? "listing-page hidden" : "listing-page"}
          aria-hidden={showingDetail || showingReader}
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
            <Settings locale={locale} setLocale={setLocale} openReader={openReader} />
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
        {showingReader && <MediumReader locale={locale} back={backFromReader} />}
      </div>
      {!showingDetail && !showingReader && <BottomNavigation tab={tab} locale={locale} onSelect={setTab} />}
      <ScrollToTop locale={locale} compact={showingDetail || showingReader} />
    </main>
  );
}
