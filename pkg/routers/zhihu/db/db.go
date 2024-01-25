package db

type DB interface {
	DataBaseObject
}

type DataBaseObject interface {
	// Save object info to zsxq_object table
	SaveObjectInfo(o *Object) error
	// Get object info from zsxq_object table
	GetObjectInfo(oid int) (o *Object, err error)
}
