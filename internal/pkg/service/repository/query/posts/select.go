package posts

// Select ...
type Select struct {
	IDs       []int64
	GroupIDs  []int64
	VKPostIDs []int64

	Published *bool

	DateFrom *int64
	DateTo   *int64
}
