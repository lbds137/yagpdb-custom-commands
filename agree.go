{{ $agreeRole := 700913447595933748 }}
{{ $information := 669842179950116864 }}
{{ $modLog := 637455918782742528 }}
{{ $welcome := 609856712333197332 }}

{{ if not (hasRoleID $agreeRole) }}
  {{ giveRoleID .User.ID $agreeRole}}

  {{ $key := "User Agreement Record" }}
  {{ $value := (joinStr "" "✅ User **" .User.String "** (ID: " .User.ID ") has agreed to abide by the rules and was given the <@&" $agreeRole "> role!") }}
  {{ execCC 3 nil 0 (sdict "Key" $key "Value" $value "Channel" $modLog) }}

  {{ $message := (joinStr "" 
      "Welcome to **" .Guild.Name "**, <@" .User.ID ">!\n\n"
      "To get started, **__please read__** <#" $information "> for instructions outlining the necessary steps for gaining access to the server as a full member.\n\n"
      "We hope you enjoy your stay!"
  )}}
  {{ sendMessage $welcome $message }}
  {{ deleteTrigger 5 }}
{{ else }}
  You have already agreed to the rules!
  {{ deleteTrigger 5 }}
  {{ deleteResponse 5 }}
{{ end }}