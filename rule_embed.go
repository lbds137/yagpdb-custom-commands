{{ $gIcon := (joinStr "" "https://cdn.discordapp.com/icons/" (toString .Guild.ID) "/" .Guild.Icon ".gif") }}

{{ $embed := cembed
    "title" (joinStr "" "Rule #" .ExecData.RuleNumber)
    "description" .ExecData.RuleText
    "color" 0xff0000
    "author" (sdict "name" .Guild.Name "url" "https://thenighthouse.org/" "icon_url" $gIcon)
}}

{{ sendMessage nil $embed }}