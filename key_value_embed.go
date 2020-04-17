{{ $gIcon := (joinStr "" "https://cdn.discordapp.com/icons/" (toString .Guild.ID) "/" .Guild.Icon ".gif") }}

{{ $embed := cembed
    "title" .ExecData.Key
    "description" .ExecData.Value
    "color" (toInt (dbGet .Guild.OwnerID "Embed Color").Value)
    "author" (sdict "name" .Guild.Name "url" "https://thenighthouse.org/" "icon_url" $gIcon)
}}

{{ sendMessage nil $embed }}