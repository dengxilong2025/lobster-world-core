package sim

// Intent is a high-level goal submission from an agent/human.
// v0 keeps this minimal. The simulation core decides how to execute it.
type Intent struct {
	Goal        string
	Constraints []string
	Horizon     string
	Risk        string
	Notes       string
}

