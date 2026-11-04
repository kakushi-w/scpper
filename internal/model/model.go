package model

import "time"

type ArticleBrief struct {
	ID        int64      `json:"id"`
	Wiki      string     `json:"wiki"`
	Page      string     `json:"page"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Rating    float64    `json:"rating"`
	Upvotes   int        `json:"upvotes"`
	Downvotes int        `json:"downvotes"`
	Comments  int        `json:"comments"`
	Author    string     `json:"author"`
	Tags      []string   `json:"tags"`
	CreatedAt *time.Time `json:"createdAt"`
	Lastmod   *time.Time `json:"lastmod"`
	Status    string     `json:"status"`
	PageID    *int64     `json:"pageId"`
}

type ArticleDetail struct {
	ArticleBrief
	RatingVotes   int        `json:"ratingVotes"`
	RevisionCount int        `json:"revisionCount"`
	EditorCount   int        `json:"editorCount"`
	DeletedAt     *time.Time `json:"deletedAt"`
	HasSource     bool       `json:"hasSource"`
	SourceAt      *time.Time `json:"sourceFetchedAt"`
	Favorited     bool       `json:"favorited"`
	FavoriteCount int        `json:"favoriteCount"`
	SyncedAt      time.Time  `json:"syncedAt"`
}

type SearchHit struct {
	ArticleBrief
	Snippet string `json:"snippet"`
}

type RatingPoint struct {
	At           time.Time `json:"at"`
	Rating       int       `json:"rating"`
	Delta        int       `json:"delta"`
	Username     string    `json:"username"`
	InitialIndex bool      `json:"initialIndex"`
}

type VoteRow struct {
	UID          string     `json:"uid"`
	Username     string     `json:"username"`
	Vote         int        `json:"vote"`
	VotedAt      *time.Time `json:"votedAt"`
	InitialIndex bool       `json:"initialIndex"`
}

type RevisionRow struct {
	RevNo     int        `json:"revNo"`
	RevID     *int64     `json:"revId"`
	Flags     string     `json:"flags"`
	UID       string     `json:"uid"`
	Username  string     `json:"username"`
	Comment   string     `json:"comment"`
	ChangedAt *time.Time `json:"changedAt"`
}

type AuthorSummary struct {
	Name          string     `json:"name"`
	UID           string     `json:"uid"`
	AvatarURL     string     `json:"avatarUrl"`
	PageCount     int        `json:"pageCount"`
	TotalRating   int        `json:"totalRating"`
	PositiveTotal int        `json:"positiveTotal"`
	NegativeTotal int        `json:"negativeTotal"`
	TotalUpvotes  int        `json:"totalUpvotes"`
	TotalDown     int        `json:"totalDownvotes"`
	TotalComments int        `json:"totalComments"`
	FirstPageAt   *time.Time `json:"firstPageAt"`
	LastPageAt    *time.Time `json:"lastPageAt"`

	RankOverall  *int `json:"rankOverall"`
	RankPositive *int `json:"rankPositive"`
	RankNegative *int `json:"rankNegative"`
	RankUpvotes  *int `json:"rankUpvotes"`
	RankDown     *int `json:"rankDownvotes"`

	Registered  bool    `json:"registered"`
	IsMember    bool    `json:"isMember"`
	MemberTitle *string `json:"memberTitle"`
	Coins       *int64  `json:"coins"`
	RankCoins   *int    `json:"rankCoins"`
}

type HeatCell struct {
	Date  string `json:"date"`
	Edits int    `json:"edits"`
	Votes int    `json:"votes"`
	Total int    `json:"total"`
}

type AuthorAffinity struct {
	Rank          int    `json:"rank"`
	UID           string `json:"uid"`
	Name          string `json:"name"`
	PositiveVotes int    `json:"positiveVotes"`
	NegativeVotes int    `json:"negativeVotes"`
	TotalScore    int    `json:"totalScore"`
}

type RecentVote struct {
	Wiki         string    `json:"wiki"`
	Page         string    `json:"page"`
	PageID       *int64    `json:"pageId"`
	Title        string    `json:"title"`
	Vote         int       `json:"vote"`
	VotedAt      time.Time `json:"votedAt"`
	InitialIndex bool      `json:"initialIndex"`
}

type AuthorRankRow struct {
	Rank         int     `json:"rank"`
	Name         string  `json:"name"`
	UID          string  `json:"uid"`
	AvatarURL    string  `json:"avatarUrl"`
	Value        int     `json:"value"`
	PageCount    int     `json:"pageCount"`
	TotalUpvotes int     `json:"totalUpvotes"`
	TotalDown    int     `json:"totalDownvotes"`
	IsMember     bool    `json:"isMember"`
	MemberTitle  *string `json:"memberTitle"`
	Registered   bool    `json:"registered"`
}

type WealthRankRow struct {
	Rank        int     `json:"rank"`
	UserID      int64   `json:"userId"`
	Name        string  `json:"name"`
	AvatarURL   *string `json:"avatarUrl"`
	Coins       int64   `json:"coins"`
	IsMember    bool    `json:"isMember"`
	MemberTitle *string `json:"memberTitle"`
}

type EditorRankRow struct {
	Rank      int    `json:"rank"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	EditCount int    `json:"editCount"`
	PageCount int    `json:"pageCount"`
}

type MeProfile struct {
	UserID      int64      `json:"userId"`
	Name        string     `json:"name"`
	UID         *string    `json:"uid"`
	AvatarURL   *string    `json:"avatarUrl"`
	Role        string     `json:"role"`
	Coins       int64      `json:"coins"`
	CoinRank    *int       `json:"coinRank"`
	IsMember    bool       `json:"isMember"`
	MemberTitle *string    `json:"memberTitle"`
	MemberSince *time.Time `json:"memberSince"`
	CreatedAt   time.Time  `json:"createdAt"`

	TotalUpvotes   int  `json:"totalUpvotes"`
	TotalDownvotes int  `json:"totalDownvotes"`
	RankUpvotes    *int `json:"rankUpvotes"`
	RankDownvotes  *int `json:"rankDownvotes"`
	TotalRating    int  `json:"totalRating"`
	RankRating     *int `json:"rankRating"`
	PageCount      int  `json:"pageCount"`
	FavoriteCount  int  `json:"favoriteCount"`
	UnreadCount    int  `json:"unreadCount"`
}

type CoinLedgerRow struct {
	ID           int64     `json:"id"`
	Delta        int64     `json:"delta"`
	BalanceAfter int64     `json:"balanceAfter"`
	Reason       string    `json:"reason"`
	Memo         *string   `json:"memo"`
	CreatedAt    time.Time `json:"createdAt"`
}

type TaskRow struct {
	Code        string     `json:"code"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	Reward      int64      `json:"reward"`
	Target      int        `json:"target"`
	Progress    int        `json:"progress"`
	Period      string     `json:"period"`
	CompletedAt *time.Time `json:"completedAt"`
	ClaimedAt   *time.Time `json:"claimedAt"`
}

type Notification struct {
	ID        int64          `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      *string        `json:"body"`
	Link      *string        `json:"link"`
	Data      map[string]any `json:"data"`
	ReadAt    *time.Time     `json:"readAt"`
	CreatedAt time.Time      `json:"createdAt"`
}

type BetOption struct {
	ID          int64   `json:"id"`
	Label       string  `json:"label"`
	TotalAmount int64   `json:"totalAmount"`
	BettorCount int     `json:"bettorCount"`
	Share       float64 `json:"share"`
	Odds        float64 `json:"odds"`
	MyAmount    int64   `json:"myAmount"`
}

type BetEvent struct {
	ID          int64       `json:"id"`
	Title       string      `json:"title"`
	Description *string     `json:"description"`
	Status      string      `json:"status"`
	ClosesAt    *time.Time  `json:"closesAt"`
	MinAmount   int64       `json:"minAmount"`
	MaxAmount   *int64      `json:"maxAmount"`
	TotalPool   int64       `json:"totalPool"`
	WinningID   *int64      `json:"winningOptionId"`
	Options     []BetOption `json:"options"`
	CreatedAt   time.Time   `json:"createdAt"`
	SettledAt   *time.Time  `json:"settledAt"`
	MyTotal     int64       `json:"myTotal"`
	MyPayout    *int64      `json:"myPayout"`
}

type MyBet struct {
	ID          int64      `json:"id"`
	EventID     int64      `json:"eventId"`
	EventTitle  string     `json:"eventTitle"`
	OptionID    int64      `json:"optionId"`
	OptionLabel string     `json:"optionLabel"`
	Amount      int64      `json:"amount"`
	Payout      *int64     `json:"payout"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	SettledAt   *time.Time `json:"settledAt"`
}

type ShopItem struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	Description   *string `json:"description"`
	ImageURL      *string `json:"imageUrl"`
	PriceCoins    *int64  `json:"priceCoins"`
	PriceCNYCents *int64  `json:"priceCnyCents"`
	MemberOnly    bool    `json:"memberOnly"`
	Stock         *int    `json:"stock"`
	Enabled       bool    `json:"enabled"`
}

type ShopOrder struct {
	ID         int64     `json:"id"`
	ItemID     int64     `json:"itemId"`
	ItemName   string    `json:"itemName"`
	Quantity   int       `json:"quantity"`
	TotalCoins int64     `json:"totalCoins"`
	Status     string    `json:"status"`
	Contact    *string   `json:"contact"`
	Note       *string   `json:"note"`
	AdminNote  *string   `json:"adminNote"`
	UserName   string    `json:"userName,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Activity struct {
	ID         int64      `json:"id"`
	Title      string     `json:"title"`
	Summary    *string    `json:"summary"`
	Body       *string    `json:"body"`
	CoverURL   *string    `json:"coverUrl"`
	Link       *string    `json:"link"`
	Status     string     `json:"status"`
	StartsAt   *time.Time `json:"startsAt"`
	EndsAt     *time.Time `json:"endsAt"`
	Pinned     bool       `json:"pinned"`
	BetEventID *int64     `json:"betEventId"`
	BetEvent   *BetEvent  `json:"betEvent,omitempty"`
}

type HomeStats struct {
	Members        int       `json:"members"`
	DailyActive    int       `json:"dailyActive"`
	TotalPages     int       `json:"totalPages"`
	TotalPosts     int       `json:"totalPosts"`
	TotalVotes     int       `json:"totalVotes"`
	TotalAuthors   int       `json:"totalAuthors"`
	TotalRevisions int       `json:"totalRevisions"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type HomePayload struct {
	Stats       HomeStats       `json:"stats"`
	Ongoing     []Activity      `json:"ongoingActivities"`
	OpenBets    []BetEvent      `json:"openBets"`
	LatestPages []ArticleBrief  `json:"latestPages"`
	TopRated    []ArticleBrief  `json:"topRatedThisMonth"`
	TopAuthors  []AuthorRankRow `json:"topAuthors"`
	TopWealth   []WealthRankRow `json:"topWealth"`
}

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Meta  any `json:"meta,omitempty"`
}
