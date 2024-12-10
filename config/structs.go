package config

type Delium_json_config struct {
	PASSWORD         string
	DELIUM_SEED_PATH string
	UID_DELETE_STEP  int
	UID_REPEAT       int
}

type Blv_info_json struct {
	PASSWORD    string
	PRIVATE_KEY string
	PUBLIC_KEY  string
	UID         string
}
