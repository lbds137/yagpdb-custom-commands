{{ $commandName := "Guest User Check" }}
{{ $hoursCutoff := 24 }}
{{ $actionTaken := "" }}

{{ $userID := (userArg (index .Args 1)).ID }}
{{ $member := getMember $userID }}
{{ $now := currentTime }}
{{ $userJoined := $member.JoinedAt }}
{{ $hoursSinceJoin := toInt ($now.Sub $userJoined.Parse).Hours }}
{{ $hoursExceeded := sub $hoursSinceJoin $hoursCutoff }}

{{ if not (targetHasRoleID $userID 624676327135641620) }} 
	{{ if gt $hoursExceeded 0 }}
		{{ $actionTaken = "kick" }}
	{{ else }}
		{{ $actionTaken = "none" }}
	{{ end }}
{{ else }}
	{{ $actionTaken = "none" }}
{{ end }}

{{ $result := "" }}
{{ if eq $actionTaken "kick" }}
	{{ $kickReason := joinStr "" "exceeding the inactivity grace period by **" $hoursExceeded "** hours" }}
	{{ $result = joinStr "" "🥾 Kicked user **" $member.User.String "** for " $kickReason "." }}
	{{ exec "kick" $userID $kickReason }}
{{ else }}
	{{ $result = joinStr ""
		"❌ No action was taken against **" $member.User.String  "** at this time."
	}}
{{ end }}

{{ execCC 3 nil 0 (sdict "Key" $commandName "Value" $result) }}
