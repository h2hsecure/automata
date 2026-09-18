package tracking

type DashboardMetrics struct {
	TotalVisitors  int64
	ActiveVisitors int64
	PageViews      int64
	TopPages       []PageViewCount
	TopReferrers   []ReferrerCount
	DeviceBreakdown map[string]int
	BrowserBreakdown map[string]int
}

type PageViewCount struct {
	URL   string
	Count int64
}

type ReferrerCount struct {
	Referrer string
	Count    int64
}
