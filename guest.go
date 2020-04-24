{{ $commandName := "Guest User Check" }}
{{ $hoursCutoff := (toInt (dbGet .Guild.OwnerID "Guest Grace Period Hours").Value) }}
{{ $actionTaken := "" }}

{{ $args := parseArgs 1 (joinStr "" "Please enter a user ID.")
    (carg "string" "user ID")
}}

{{ $userID := (userArg ($args.Get 0)).ID }}
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
        {{ $hoursString := "hours" }}
        {{ if eq $hoursExceeded 1 }}
            {{ $hoursString = "hour" }}
        {{ end }}
	{{ $kickReason := joinStr "" "exceeding the inactivity grace period by **" $hoursExceeded "** " $hoursString }}
	{{ $result = joinStr "" "🥾 Kicked user **" $member.User.String "** for " $kickReason "." }}
	{{ $silent := exec "kick" $userID $kickReason }}
{{ else }}
	{{ $result = joinStr "" "❌ No action was taken against **" $member.User.String "** at this time." }}
{{ end }}
 
{{ execCC 3 nil 0 (sdict "Title" $commandName "Description" $result) }}