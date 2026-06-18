package recycleapp

import "errors"

var (
	ErrNoJSMatch            = errors.New("main page did not contain the expected main js url")
	ErrZipcodeNoResult      = errors.New("zipcode query returned nothing")
	ErrStreetNoResult       = errors.New("street query returned nothing")
	ErrOrganizationNoResult = errors.New("organization query returned nothing")
)
