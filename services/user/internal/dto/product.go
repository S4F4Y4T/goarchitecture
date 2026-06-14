package dto

type CreateProductRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description string  `json:"description" validate:"omitempty"`
	Price       float64 `json:"price" validate:"required,min=0"`
}

// UpdateProductRequest is a full representation of the product: PUT replaces
// the entire resource, so every field is required (description may be empty).
type UpdateProductRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description string  `json:"description" validate:"omitempty"`
	Price       float64 `json:"price" validate:"required,min=0"`
}
