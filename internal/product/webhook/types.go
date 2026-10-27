package webhook

// webhookProduct represents the Shopify product webhook payload.
type webhookProduct struct {
	ID             int64            `json:"id"`
	Handle         string           `json:"handle"`
	Title          string           `json:"title"`
	BodyHTML       string           `json:"body_html"`
	Vendor         string           `json:"vendor"`
	ProductType    string           `json:"product_type"`
	Tags           string           `json:"tags"`
	Status         string           `json:"status"`
	TemplateSuffix string           `json:"template_suffix"`
	PublishedAt    *string          `json:"published_at"`
	Options        []webhookOption  `json:"options"`
	Variants       []webhookVariant `json:"variants"`
	Images         []webhookImage   `json:"images"`
}

type webhookVariant struct {
	ID                 int64   `json:"id"`
	Title              string  `json:"title"`
	SKU                string  `json:"sku"`
	Barcode            string  `json:"barcode"`
	Price              string  `json:"price"`
	CompareAtPrice     *string `json:"compare_at_price"`
	Position           int     `json:"position"`
	InventoryQuantity  int     `json:"inventory_quantity"`
	InventoryPolicy    string  `json:"inventory_policy"`
	FulfillmentService string  `json:"fulfillment_service"`
	Option1            string  `json:"option1"`
	Option2            string  `json:"option2"`
	Option3            string  `json:"option3"`
	Taxable            bool    `json:"taxable"`
	RequiresShipping   bool    `json:"requires_shipping"`
	Weight             float64 `json:"weight"`
	WeightUnit         string  `json:"weight_unit"`
	Grams              int     `json:"grams"`
}

type webhookImage struct {
	ID       int64   `json:"id"`
	Src      string  `json:"src"`
	Alt      *string `json:"alt"`
	Position int     `json:"position"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
}

type webhookOption struct {
	Name     string   `json:"name"`
	Position int      `json:"position"`
	Values   []string `json:"values"`
}

type webhookDeletePayload struct {
	ID int64 `json:"id"`
}
