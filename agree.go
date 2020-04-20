{{ $agreeRole := 700913447595933748 }}
{{ $introduction := 497573378807562250 }}
{{ $modLog := 637455918782742528 }}
 
{{ if not (hasRoleID $agreeRole) }}
  {{ giveRoleID .User.ID $agreeRole}}
 
  {{ $title := "User Agreement Record" }}
  {{ $description := (joinStr "" "✅ User **" .User.String "** (ID: " .User.ID ") has agreed to abide by the rules and was given the <@&" $agreeRole "> role!") }}
  {{ execCC 3 nil 0 (sdict "Title" $title "Description" $description "Channel" $modLog) }}
 
  ✅ Your agreement has been recorded! Please proceed to <#{{ $introduction }}> to post a compliant introduction. Thank you!
  {{ deleteTrigger 5 }}
  {{ deleteResponse 5 }}
{{ else }}
  ❌ You have already agreed to the rules!
  {{ deleteTrigger 5 }}
  {{ deleteResponse 5 }}
{{ end }}