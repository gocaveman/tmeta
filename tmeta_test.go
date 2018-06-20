package tmeta

import "testing"

type Author struct {
	AuthorID   string `db:"author_id" dbrobj:"pk"`
	NomDePlume string `db:"nom_de_plume"`

	Books []Book `db:"-" dbrobj:"has_many"`
}

type Publisher struct {
	PublisherID string `db:"publisher_id" dbrobj:"pk"`
	CompanyName string `db:"company_name"`

	Books []Book `db:"-" dbrobj:"has_many,rel_name=books"`
}

type Book struct {
	BookID string `db:"book_id" dbrobj:"pk"`

	AuthorID string  `db:"author_id" dbrobj:""`
	Author   *Author `db:"-" dbrobj:"belongs_to,rel_key=author_id"`

	PublisherID string     `db:"publisher_id" dbrobj:""`
	Publisher   *Publisher `db:"-" dbrobj:"belongs_to"`

	Title string `db:"title"`

	Categories []Category `db:"-"`

	CategoryIDs []string `db:"-" dbrobj:"belongs_to_many_ids,through=book_category"`
}

type BookCategory struct {
	BookID     string `db:"publisher_id" dbrobj:"pk"`
	CategoryID string `db:"category_id" dbrobj:"pk"`
}

type Category struct {
	CategoryID string `db:"category_id" dbrobj:"pk"`
	Name       string `db:"name"`
	Books      []Book `db:"-" dbrobj:"belongs_to_many,through=book_category"`
}

func TestDemo1(t *testing.T) {

}
