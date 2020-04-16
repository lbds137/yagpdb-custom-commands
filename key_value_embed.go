{{ $key := .ExecData.Key }}
{{ $value := (joinStr "\n" (split .ExecData.Value "\\n")) }}
{{ $gIcon := (joinStr "" "https://cdn.discordapp.com/icons/" (toString .Guild.ID) "/" .Guild.Icon ".gif") }}

{{ $embed := cembed
    "title" $key
    "description" $value
    "color" 0xff0000
    "author" (sdict "name" .Guild.Name "url" "https://thenighthouse.org/" "icon_url" $gIcon)
}}

{{ sendMessage nil $embed }}