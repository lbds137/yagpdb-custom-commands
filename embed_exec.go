{{ $author := "" }}
{{ if and .ExecData.AuthorID (ne (toString .ExecData.AuthorID) (toString .Guild.ID)) }}
    {{ $member := getMember .ExecData.AuthorID }}
    {{ $userFull := $member.User.String }}
    {{ if $member.Nick }}
        {{ $userFull = joinStr "" $member.Nick " (" $member.User.String ")" }}
    {{ end }}
    {{ $userLink := joinStr "" "https://discordapp.com/users/" $member.User.ID }}
    {{ $uAvatar := joinStr "" "https://cdn.discordapp.com/avatars/" $member.User.ID "/" $member.User.Avatar ".gif" }}
    {{ $author = sdict "name" $userFull "url" $userLink "icon_url" $uAvatar }}
{{ else }}
    {{ $gIcon := (joinStr "" "https://cdn.discordapp.com/icons/" (toString .Guild.ID) "/" .Guild.Icon ".gif") }}
    {{ $author = sdict "name" .Guild.Name "url" "https://thenighthouse.org/" "icon_url" $gIcon }}
{{ end }}

{{ $dbEmbedColor :=  toInt (dbGet .Guild.OwnerID "Embed Color").Value }}

{{ $embed := cembed
    "title" .ExecData.Title
    "description" .ExecData.Description
    "color" (or .ExecData.Color $dbEmbedColor)
    "author" $author
    "image" (sdict "url" .ExecData.ImageURL)
}}

{{ sendMessage .ExecData.Channel $embed }}