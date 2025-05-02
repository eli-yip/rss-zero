package db

//	"author": {
//	  "name": "墨苍离",
//	  "url_token": "canglimo"
//	}
type Author struct {
	ID   string `gorm:"column:id;type:text;primary_key"` // url_token in zhihu api resp
	Name string `gorm:"column:name;type:text"`
}

func (a *Author) TableName() string { return "zhihu_author" }

type DBAuthor interface {
	// Save author info to zhihu_author table
	SaveAuthor(a *Author) error
	// Get author name
	GetAuthorName(id string) (name string, err error)
}

func (d *DBService) SaveAuthor(a *Author) error {
	return d.Save(a).Error
}

func (d *DBService) GetAuthorName(id string) (name string, err error) {
	a := &Author{}
	err = d.Where("id = ?", id).First(a).Error
	return a.Name, err
}
