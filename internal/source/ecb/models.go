package ecb

// APIResponse represents the ECB API response structure.
type APIResponse struct {
	PageInfo PageInfo  `json:"pageInfo"`
	Content  []Content `json:"content"`
}

type PageInfo struct {
	Page       int `json:"page"`
	NumPages   int `json:"numPages"`
	PageSize   int `json:"pageSize"`
	NumEntries int `json:"numEntries"`
}

type Content struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	Description  *string    `json:"description"`
	Date         string     `json:"date"`
	CanonicalURL string     `json:"canonicalUrl"`
	Body         *string    `json:"body"`
	Tags         []APITag   `json:"tags"`
	LeadMedia    *LeadMedia `json:"leadMedia"`
	Summary      *string    `json:"summary"`
	Author       *string    `json:"author"`
	Duration     int        `json:"duration"`
	LastModified int64      `json:"lastModified"`
}

type APITag struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

type LeadMedia struct {
	ImageURL string `json:"imageUrl"`
}
