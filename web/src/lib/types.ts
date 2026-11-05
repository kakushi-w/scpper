export interface PageMeta {
  total: number;
  page: number;
  pageSize: number;
  hasNext: boolean;
}

export interface ArticleBrief {
  id: number;
  wiki: string;
  page: string;
  title: string;
  url: string;
  rating: number;
  upvotes: number;
  downvotes: number;
  comments: number;
  author: string;
  tags: string[];
  createdAt: string | null;
  lastmod: string | null;
  status: string;
  pageId: number | null;
}

export interface ArticleDetail extends ArticleBrief {
  ratingVotes: number;
  revisionCount: number;
  editorCount: number;
  deletedAt: string | null;
  hasSource: boolean;
  sourceFetchedAt: string | null;
  favorited: boolean;
  favoriteCount: number;
  syncedAt: string;
}

export interface SlugMatch {
  id: number;
  pageId: number | null;
  title: string;
  status: string;
  author: string;
  rating: number;
  upvotes: number;
  downvotes: number;
  comments: number;
  tags: string[];
  createdAt: string | null;
  lastmod: string | null;
  deletedAt: string | null;
  deletedAtInitial: boolean;
}

export interface DeletedPageRow extends ArticleBrief {
  deletedAt: string | null;
  slugTaken: boolean;
  deletedAtInitial: boolean;
}

export interface SearchHit extends ArticleBrief {
  snippet: string;
}

export interface SearchResponse {
  query: string;
  items: SearchHit[];
  meta: PageMeta;
  tookMs: number;
}

export interface RatingPoint {
  initialIndex: boolean;
  at: string;
  rating: number;
  delta: number;
  username: string;
}

export interface VoteRow {
  uid: string;
  username: string;
  vote: number;
  votedAt: string | null;
  initialIndex: boolean;
}

export interface RevisionRow {
  revNo: number;
  revId: number | null;
  flags: string;
  uid: string;
  username: string;
  comment: string;
  changedAt: string | null;
}

export interface AuthorSummary {
  name: string;
  uid: string;
  avatarUrl: string;
  pageCount: number;
  totalRating: number;
  positiveTotal: number;
  negativeTotal: number;
  totalUpvotes: number;
  totalDownvotes: number;
  totalComments: number;
  firstPageAt: string | null;
  lastPageAt: string | null;
  rankOverall: number | null;
  rankPositive: number | null;
  rankNegative: number | null;
  rankUpvotes: number | null;
  rankDownvotes: number | null;
  registered: boolean;
  isMember: boolean;
  memberTitle: string | null;
  coins: number | null;
  rankCoins: number | null;
}

export interface HeatCell {
  date: string;
  edits: number;
  votes: number;
  total: number;
}

export interface AuthorAffinity {
  rank: number;
  uid: string;
  name: string;
  positiveVotes: number;
  negativeVotes: number;
  totalScore: number;
}

export interface RecentVote {
  initialIndex: boolean;
  pageId: number | null;
  wiki: string;
  page: string;
  title: string;
  vote: number;
  votedAt: string;
}

export interface AuthorRankRow {
  rank: number;
  name: string;
  uid: string;
  avatarUrl: string;
  value: number;
  pageCount: number;
  totalUpvotes: number;
  totalDownvotes: number;
  isMember: boolean;
  memberTitle: string | null;
  registered: boolean;
}

export interface WealthRankRow {
  rank: number;
  pwikitId: number;
  name: string;
  avatarUrl?: string | null;
  coins: number;
}

export interface EditorRankRow {
  avatarUrl: string;
  rank: number;
  uid: string;
  name: string;
  editCount: number;
  pageCount: number;
}

export interface SessionUser {
  id: number;
  pwikitId: number;
  name: string;
  avatarUrl: string | null;
  role: string;
  coins: number;
  createdAt: string;
}

export interface Standing {
  pwikitId: number;
  name: string;
  coins: number;
  rank: number | null;
  ranked: number;
}

export interface PageTips {
  pageId: number;
  title: string;
  authors: string[];
  total: number;
  count: number;
}

export type TaskMetric =
  | "vote"
  | "vote_up"
  | "vote_down"
  | "post"
  | "comment"
  | "thread_post"
  | "edit"
  | "page"
  | "page_score"
  | "comments_received"
  | "score_received"
  | "tip_given"
  | "tip_received"
  | "gacha"
  | "trade";

export interface AdminTask {
  code: string;
  kind: "daily" | "weekly" | "monthly";
  metric: TaskMetric;
  tag: string;
  category: string;
  score: number | null;
  title: string;
  description: string | null;
  reward: number;
  target: number;
  sort: number;
  enabled: boolean;
}

export interface MeProfile {
  userId: number;
  name: string;
  uid: string | null;
  avatarUrl: string | null;
  role: string;
  coins: number;
  coinRank: number | null;
  isMember: boolean;
  memberTitle: string | null;
  memberSince: string | null;
  createdAt: string;
  totalUpvotes: number;
  totalDownvotes: number;
  rankUpvotes: number | null;
  rankDownvotes: number | null;
  totalRating: number;
  rankRating: number | null;
  pageCount: number;
  favoriteCount: number;
  unreadCount: number;
}

export interface CoinLedgerRow {
  id: number;
  delta: number;
  balanceAfter: number;
  reason: string;
  memo: string | null;
  createdAt: string;
}

export interface TaskRow {
  code: string;
  kind: "daily" | "weekly" | "monthly";
  title: string;
  description: string | null;
  reward: number;
  target: number;
  progress: number;
  period: string;
  completedAt: string | null;
  claimedAt: string | null;
}

export interface TasksResponse {
  daily: TaskRow[];
  weekly: TaskRow[];
  monthly: TaskRow[];
  dailyPeriod: string;
  weeklyPeriod: string;
  monthlyPeriod: string;
  claimableCoins: number;
}

export interface Notification {
  id: number;
  type: string;
  title: string;
  body: string | null;
  link: string | null;
  data: Record<string, unknown>;
  readAt: string | null;
  createdAt: string;
}

export interface BetOption {
  id: number;
  label: string;
  totalAmount: number;
  bettorCount: number;
  share: number;
  odds: number;
  myAmount: number;
}

export interface BetEvent {
  id: number;
  title: string;
  description: string | null;
  status: "open" | "locked" | "settled" | "cancelled";
  closesAt: string | null;
  minAmount: number;
  maxAmount: number | null;
  totalPool: number;
  winningOptionId: number | null;
  options: BetOption[];
  createdAt: string;
  settledAt: string | null;
  myTotal: number;
  myPayout: number | null;
}

export interface MyBet {
  id: number;
  eventId: number;
  eventTitle: string;
  optionId: number;
  optionLabel: string;
  amount: number;
  payout: number | null;
  status: string;
  createdAt: string;
  settledAt: string | null;
}

export interface ShopItem {
  id: number;
  name: string;
  description: string | null;
  imageUrl: string | null;
  priceCoins: number | null;
  priceCnyCents: number | null;
  memberOnly: boolean;
  stock: number | null;
  enabled: boolean;
}

export interface ShopOrder {
  id: number;
  itemId: number;
  itemName: string;
  quantity: number;
  totalCoins: number;
  status: string;
  contact: string | null;
  note: string | null;
  adminNote: string | null;
  userName?: string;
  createdAt: string;
}

export interface Activity {
  id: number;
  title: string;
  summary: string | null;
  body: string | null;
  coverUrl: string | null;
  link: string | null;
  status: "upcoming" | "ongoing" | "ended";
  startsAt: string | null;
  endsAt: string | null;
  pinned: boolean;
  betEventId: number | null;
  betEvent?: BetEvent;
}

export interface HomeStats {
  members: number;
  dailyActive: number;
  totalPages: number;
  totalPosts: number;
  totalVotes: number;
  totalAuthors: number;
  totalRevisions: number;
  updatedAt: string;
}

export interface HomePayload {
  stats: HomeStats;
  ongoingActivities: Activity[];
  openBets: BetEvent[];
  latestPages: ArticleBrief[];
  topRatedThisMonth: ArticleBrief[];
  topAuthors: AuthorRankRow[];
  topWealth: WealthRankRow[];
}

export interface WikiRow {
  id: string;
  name: string;
  siteUrl: string;
  isPrimary: boolean;
  pageCount: number;
}

export interface ListResponse<T> {
  items: T[];
  meta: PageMeta;
}

export type Rarity = "ur" | "ssr" | "sr" | "r" | "n";

export interface GachaCard {
  articleId: number;
  wiki: string;
  page: string;
  pageId: number | null;
  title: string;
  author: string;
  rating: number;
  tags: string[];
  rarity: Rarity;
  byPity: boolean;
  isNew: boolean;
}

export interface OwnedCard extends GachaCard {
  qty: number;
  firstAt: string;
}

export interface GachaInfo {
  costSingle: number;
  costTen: number;
  pityCount: number;
  tenGuarantee: Rarity;
  tradeFeePct: number;
  tiers: { rarity: Rarity; rate: number; pool: number }[];
  me?: {
    totalPulls: number;
    sinceSSR: number;
    pityIn: number;
    ownedTotal: number;
    ownedKinds: number;
    poolKinds: number;
    balanceCoins: number;
  };
}

export interface GachaListing extends GachaCard {
  id: number;
  price: number;
  createdAt: string;
  seller: string;
}

export interface MyListing extends GachaCard {
  id: number;
  price: number;
  status: string;
  createdAt: string;
  closedAt: string | null;
}

export interface PullRecord {
  rarity: Rarity;
  byPity: boolean;
  batch: string;
  createdAt: string;
  wiki: string;
  page: string;
  pageId: number | null;
  title: string;
  author: string;
  rating: number;
}

export interface StockSymbol {
  code: string;
  name: string;
  shortName: string;
  source: "local" | "crom";
  siteUrl?: string;
  listedAt?: string | null;
  price: number;
  fair: number;
  dayOpen: number;
  changePct: number;
  dayVolume: number;
  myQty: number;
}

export interface StockHolding {
  symbol: string;
  name: string;
  shortName: string;
  qty: number;
  avgCost: number;
  price: number;
  margin: number;
  lockedQty: number;
  marketValue: number;
  realizedPnl: number;
  floatingPnl: number;
  equity: number;
  marginRatio: number;
  liqPrice: number;
}

export interface StockPortfolio {
  items: StockHolding[];
  total: {
    realizedPnl: number;
    floatingPnl: number;
    margin: number;
    balance: number;
  };
}

export interface AuctionEvent {
  title: string;
  altTitle?: string;
  url: string;
  createdAt: string;
}

export interface StockAuction {
  at: string;
  priceBefore: number;
  priceAfter: number;
  changePct: number;
  fair: number;
  orders: number;
  buyQty: number;
  sellQty: number;
  impactPct: number;
  driftPct: number;
  shockPct: number;
  eventPct: number;
  newPages: number;
  expected: number;
  events: AuctionEvent[];
}

export interface PendingOrder {
  id: number;
  side: "buy" | "sell";
  qty: number;
  leverage: number;
  hold: number;
  createdAt: string;
}

export interface StockQuote {
  code: string;
  name: string;
  shortName: string;
  source: "local" | "crom";
  siteUrl?: string;
  listedAt?: string | null;
  price: number;
  fair: number;
  prevClose: number;
  changePct: number;
  dayVolume: number;
  floatShares: number;
  feePct: number;
  bandPct: number;
  maxPosition: number;
  impactPct: number;
  updatedAt: string;
  auctionTimes: string[];
  nextAuctionAt: string;
  capPrice: number;
  floorPrice: number;
  maxLeverage: number;
  maintenancePct: number;
  borrowFeePct: number;
  allowShort: boolean;
  pending: {
    orders: number;
    buyQty: number;
    sellQty: number;
    estImpactPct: number;
  };
  lastAuction?: StockAuction;
  me?: {
    qty: number;
    avgCost: number;
    marketValue: number;
    realizedPnl: number;
    floatingPnl: number;
    balanceCoins: number;
    margin: number;
    equity: number;
    notional: number;
    marginRatio?: number;
    liqPrice?: number;
    leverage?: number;
    pendingOrder?: PendingOrder;
  };
}

export type CandleSpan = "tick" | "day" | "week" | "month" | "year";

export interface Candle {
  at: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export interface StockJob {
  kind: string;
  target: string;
  done: number;
  total: number;
  startedAt: string;
  finishedAt?: string;
  error?: string;
}

export interface StockTrade {
  symbol: string;
  side: "buy" | "sell";
  qty: number;
  price: number;
  gross: number;
  fee: number;
  at: string;
}

export interface ForumCategory {
  id: number;
  title: string;
  description: string;
  threads: number;
  posts: number;
  lastAt: string | null;
}

export interface ForumThreadRow {
  id: number;
  categoryId: number;
  category: string;
  title: string;
  description: string;
  startedAt: string | null;
  startedBy: string;
  startedUid: string | null;
  lastAt: string | null;
  lastBy: string;
  postsNum: number;
  sticky: boolean;
  locked: boolean;
}

export interface ForumPost {
  id: number;
  parentId: number | null;
  title: string;
  authorUid: string | null;
  author: string;
  avatarUrl: string;
  createdAt: string | null;
  editedAt: string | null;
  html: string;
  depth: number;
}

export interface ForumThreadDetail {
  id: number;
  categoryId: number;
  category: string;
  title: string;
  description: string;
  startedAt: string | null;
  startedBy: string;
  lastAt: string | null;
  postsNum: number;
  sticky: boolean;
  locked: boolean;
  syncedAt: string;
  posts: ForumPost[];
}

export interface ForumSearchHit {
  postId: number;
  threadId: number;
  thread: string;
  categoryId: number;
  category: string;
  author: string;
  authorUid: string | null;
  createdAt: string | null;
  title: string;
  snippet: string;
}

export type StockParams = {
  floatShares: number;
  basePrice: number;
  scaleWeight: number;
  scaleBase: number;
  sigmaDaily: number;
  halfLifeDays: number;
  feePct: number;
  dailyBandPct: number;
  maxPositionPct: number;
  minPrice: number;
  maxLeverage: number;
  maintenanceMarginPct: number;
  borrowFeeDailyPct: number;
  allowShort: boolean;
  auctionTimes: string[];
  impactPct: number;
  eventPct: number;
  eventCapPct: number;
};

export type AdminStockPlayer = {
  userId: number;
  name: string;
  qty: number;
  avgCost: number;
  realizedPnl: number;
  floatingPnl: number;
  totalPnl: number;
};

export type AdminSymbolRow = {
  code: string;
  name: string;
  shortName: string;
  source: "local" | "crom";
  siteUrl?: string;
  enabled: boolean;
  listedAt?: string | null;
  price: number;
  fair: number;
  scaleBase: number;
  mirrorPages: number;
  lastPageAt?: string | null;
  lastVoteAt?: string | null;
  sweptAt?: string | null;
  upstreamStale: boolean;
};

export type AdminStockState = {
  symbol: string;
  name: string;
  source: "local" | "crom";
  params: StockParams;
  overlay: StockParams;
  symbols: AdminSymbolRow[];
  shares: {
    positions: number;
    maker: number;
    bots: number;
    total: number;
    float: number;
    drift: number;
  };
  book: {
    inventory: number;
    cash: number;
    reserve: number;
    inventoryValue: number;
    netWorth: number;
    pRef: number;
    pFair: number;
  };
  players: {
    holders: number;
    sharesHeld: number;
    realizedPnl: number;
    floatingPnl: number;
    feesPaid: number;
    netTransfer: number;
    turnoverDay: number;
    openOrders: number;
  };
  top: AdminStockPlayer[];
  auctions: StockAuction[];
};


export interface GachaParams {
  costSingle: number;
  costTen: number;
  rates: Record<string, number>;
  tenGuarantee: string;
  pityCount: number;
  tradeFeePct: number;
}

export interface AdminLedgerRow {
  id: number;
  userId: number;
  name: string;
  delta: number;
  balanceAfter: number;
  reason: string;
  memo: string | null;
  operator: string | null;
  createdAt: string;
}

export interface AdminLedger extends ListResponse<AdminLedgerRow> {
  income: number;
  expense: number;
}

export interface TaskCompletion {
  userId: number;
  name: string;
  progress: number;
  target: number;
  completedAt: string | null;
  claimedAt: string | null;
}

export interface TaskCompletions extends ListResponse<TaskCompletion> {
  period: string;
  periods: { period: string; completed: number; claimed: number }[];
}

export interface StockHolder {
  userId: number;
  name: string;
  qty: number;
  lockedQty: number;
  avgCost: number;
  margin: number;
  realizedPnl: number;
  value: number;
}
