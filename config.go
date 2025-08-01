package modsimpro

type Serial struct {
	Url      string  `json:"url"`
	Timeout  int     `json:"timeout"`
	Speed    int     `json:"speed"`
	DataBits int     `json:"data_bits"`
	Parity   int     `json:"parity"`
	StopBits int     `json:"stop_bits"`
	Slaves   []Slave `json:"slaves"`
}

type Slave struct {
	Address uint8  `json:"address,omitempty"`
	Name    int    `json:"name"`
	Type    string `json:"type"`
}

type Config struct {
	Serial []Serial `json:"serial"`
}
