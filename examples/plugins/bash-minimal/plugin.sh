#!/usr/bin/env bash
set -euo pipefail

emit() {
  printf '%s\n' "$1"
}

emit '{"type":"handshake","protocol_version":"v2","plugin_name":"bash-minimal","capabilities":{"ops":["config.mutate"]}}'

while IFS= read -r line; do
  op="$(printf '%s' "$line" | jq -r '.op')"
  rid="$(printf '%s' "$line" | jq -r '.request_id')"

  if [[ "$op" == "config.mutate" ]]; then
    emit "$(jq -n --arg rid "$rid" '{
      type:"response", request_id:$rid, ok:true,
      output:{config_patch:{set:{"services.demo.port":1234}, unset:[]}}
    }')"
  else
    emit "$(jq -n --arg rid "$rid" --arg op "$op" '{
      type:"response", request_id:$rid, ok:false,
      error:{code:"E_UNSUPPORTED", message:("unsupported op: "+$op)}
    }')"
  fi
done
