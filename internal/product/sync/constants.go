package sync

const (
	// GramsPerPound is the conversion factor from pounds to grams.
	GramsPerPound = 453.592

	// GramsPerOunce is the conversion factor from ounces to grams.
	GramsPerOunce = 28.3495

	// GramsPerKilogram is the conversion factor from kilograms to grams.
	GramsPerKilogram = 1000

	// PresignGetExpirySec is how long a presigned GET URL remains valid for image sync (15 minutes).
	PresignGetExpirySec = 900

	// shopifyPageSize is the number of products fetched per Shopify API page.
	shopifyPageSize = 50
)
