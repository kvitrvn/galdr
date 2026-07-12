package i18n

import (
	"regexp"
	"slices"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name       string
		configured Language
		env        map[string]string
		want       Language
	}{
		{name: "config wins", configured: German, env: map[string]string{"LC_ALL": "fr_FR.UTF-8"}, want: German},
		{name: "LC_ALL", configured: Auto, env: map[string]string{"LC_ALL": "es_MX", "LC_MESSAGES": "fr"}, want: Spanish},
		{name: "LC_MESSAGES", configured: Auto, env: map[string]string{"LC_MESSAGES": "de-DE", "LANG": "fr"}, want: German},
		{name: "LANG", configured: Auto, env: map[string]string{"LANG": "fr_FR.UTF-8"}, want: French},
		{name: "C", configured: Auto, env: map[string]string{"LANG": "C"}, want: English},
		{name: "POSIX", configured: Auto, env: map[string]string{"LANG": "POSIX"}, want: English},
		{name: "encoding and modifier", configured: Auto, env: map[string]string{"LANG": "de_DE.UTF-8@euro"}, want: German},
		{name: "unsupported highest priority", configured: Auto, env: map[string]string{"LC_ALL": "it_IT", "LANG": "fr_FR"}, want: English},
		{name: "unset", configured: Auto, env: map[string]string{}, want: English},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Resolve(tt.configured, func(name string) string { return tt.env[name] })
			if got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCataloguesAreCompleteAndFormattingCompatible(t *testing.T) {
	placeholder := regexp.MustCompile(`%(?:\[[0-9]+\])?[-+#0 ']*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)
	for _, language := range []Language{English, French, Spanish, German} {
		language := language
		t.Run(string(language), func(t *testing.T) {
			t.Parallel()
			catalogue := catalogues[language]
			for _, key := range Keys() {
				message, ok := catalogue[key]
				if !ok || message == "" {
					t.Errorf("catalogue %s missing %q", language, key)
					continue
				}
				want := placeholder.FindAllString(english[key], -1)
				got := placeholder.FindAllString(message, -1)
				if !slices.Equal(got, want) {
					t.Errorf("catalogue %s key %q placeholders = %v, want %v", language, key, got, want)
				}
			}
		})
	}
}

func TestTranslatorFallsBackPerKey(t *testing.T) {
	tr := Translator{language: French, catalog: map[Key]string{Library: "Bibliothèque"}}
	if got := tr.T(Library); got != "Bibliothèque" {
		t.Errorf("translated key = %q", got)
	}
	if got := tr.T(Queue); got != "Queue" {
		t.Errorf("fallback key = %q, want English", got)
	}
}

func TestPluralPhrases(t *testing.T) {
	for _, language := range []Language{English, French, Spanish, German} {
		tr := New(language)
		pairs := []struct {
			name     string
			singular Key
			plural   Key
			argsOne  []any
			argsMany []any
		}{
			{name: "results", singular: ResultsOne, plural: ResultsOther, argsOne: []any{1, 3}, argsMany: []any{2, 3}},
			{name: "tracks", singular: StatusLoadedOne, plural: StatusLoadedOther, argsOne: []any{1}, argsMany: []any{2}},
			{name: "unavailable", singular: UnavailableOne, plural: UnavailableOther, argsOne: []any{1}, argsMany: []any{2}},
		}
		for _, pair := range pairs {
			one := tr.N(1, pair.singular, pair.plural, pair.argsOne...)
			many := tr.N(2, pair.singular, pair.plural, pair.argsMany...)
			if one == many {
				t.Errorf("%s %s singular and plural are identical: %q", language, pair.name, one)
			}
		}
	}
}
