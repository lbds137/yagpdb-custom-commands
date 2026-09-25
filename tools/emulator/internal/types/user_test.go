package types

import "testing"

// DiscordUser's String and AvatarURL are discordgo's User methods
func TestUserStringAndAvatar(t *testing.T) {
	// the new username system's default avatar is (id >> 22) % 6; the legacy one's
	// discriminator % 5
	newSystem := DiscordUser{ID: 9 << 22, Username: "new", Discriminator: "0"}
	legacy := DiscordUser{ID: 5, Username: "old", Discriminator: "0007"}
	cases := []struct{ got, want string }{
		{newSystem.String(), "new"},
		{legacy.String(), "old#0007"},
		{newSystem.AvatarURL(""), "https://cdn.discordapp.com/embed/avatars/3.png"}, // 9 % 6
		{legacy.AvatarURL("32"), "https://cdn.discordapp.com/embed/avatars/2.png?size=32"},
		{DiscordUser{ID: 7, Avatar: "abc"}.AvatarURL(""), "https://cdn.discordapp.com/avatars/7/abc.png"},
		{DiscordUser{ID: 7, Avatar: "a_abc"}.AvatarURL("256"), "https://cdn.discordapp.com/avatars/7/a_abc.gif?size=256"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: got %q, want %q", i, c.got, c.want)
		}
	}
}
