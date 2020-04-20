{{ $gIcon := (joinStr "" "https://cdn.discordapp.com/icons/" (toString .Guild.ID) "/" .Guild.Icon ".gif") }}
{{ $dbEmbedColor :=  toInt (dbGet .Guild.OwnerID "Embed Color").Value }}

{{ $embed := cembed
    "title" .ExecData.Key
    "description" .ExecData.Value
    "color" (or .ExecData.Color $dbEmbedColor)
    "author" (sdict "name" .Guild.Name "url" "https://thenighthouse.org/" "icon_url" $gIcon)
    "image" (sdict "url" .ExecData.ImageURL)
}}

{{ sendMessage .ExecData.Channel $embed }}