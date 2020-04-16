{{ $trigger := .Message.Content }}
{{ $tcID := slice $trigger 2 (sub (len $trigger) 1) }}
{{ $thereChannel := getChannel $tcID }}
{{ if $thereChannel }}
    {{ $hereMsgID := sendMessageRetID nil (complexMessage "content" "Loading...") }}
    {{ $thereMsgID := sendMessageRetID $tcID (complexMessage "content" "Loading...") }}

    {{ $cLinkBaseUrl := "https://discordapp.com/channels" }}
    {{ $thereLink := joinStr "" $cLinkBaseUrl "/" .Guild.ID "/" $tcID "/" $thereMsgID }}
    {{ $hereLink := joinStr "" $cLinkBaseUrl "/" .Guild.ID "/" .Channel.ID "/" $hereMsgID }}

    {{ $hereText := "" }}
    {{ $thereText := "" }}
    {{ $staffRoleID := 499037360340598819 }}
    {{ if hasRoleID $staffRoleID }}
        {{ $hereText = joinStr ""
            "A staff member has moved this conversation to **#" $thereChannel.Name "**. Please go there now.\n\n"
            "🔗 [Click or tap here to move.](" $thereLink ")"
        }}
        {{ $thereText = joinStr ""
            "A staff member has moved a conversation from **#" .Channel.Name "** to here.\n\n"
            "🔗 [Click or tap here to return.](" $hereLink ")"
        }}
    {{ else }}
        {{ $hereText = joinStr ""
            "A server member has linked to **#" $thereChannel.Name "**.\n\n"
            "🔗 [Click or tap here to move.](" $thereLink ")"
        }}
        {{ $thereText = joinStr ""
            "A server member has linked from **#" .Channel.Name "** to here.\n\n"
            "🔗 [Click or tap here to return.](" $hereLink ")"
        }}
    {{ end }}
    
    {{ $title := "Channel Link" }}
    {{ $color := 0xff0000 }}

    {{ $userFull := .User.String }}
    {{ if .Member.Nick }}
        {{ $userFull = joinStr "" .Member.Nick " (" .User.String ")" }}
    {{ end }}
    {{ $userLink := joinStr "" "https://discordapp.com/users/" .User.ID }}
    {{ $uAvatar := joinStr "" "https://cdn.discordapp.com/avatars/" .User.ID "/" .User.Avatar ".gif" }}
    {{ $author := sdict "name" $userFull "url" $userLink "icon_url" $uAvatar }}

    {{ editMessage nil $hereMsgID (complexMessageEdit
        "content" ""
        "embed" (cembed
            "title" $title
            "description" $hereText
            "color" $color
            "author" $author
        )
    ) }}
    {{ editMessage $tcID $thereMsgID (complexMessageEdit
        "content" ""
        "embed" (cembed
            "title" $title
            "description" $thereText
            "color" $color
            "author" $author
        )
    ) }}

    {{ deleteTrigger 0 }}
{{ end }}