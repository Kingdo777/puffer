#!/bin/bash

# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"
# get project root path
project_root_path="$(dirname "$current_script_path")"
# get scripts path
scripts_path="$project_root_path/scripts"

new_proxy_value=$1
if [ -z "$new_proxy_value" ]; then
  new_proxy_value=""
fi

search_and_replace() {
  local file="$1"
  local new_proxy="$2"

  if [[ -f "$file" ]]; then
    sed -i "s|https_proxy=http://ip:port|$new_proxy|g" "$file"
  fi
}

recursive_search() {
  local dir="$1"
  local new_proxy="$2"

  for item in "$dir"/*; do
    if [[ -d "$item" ]]; then
      recursive_search "$item" "$new_proxy"
    elif [[ -f "$item" && "$item" == *.sh ]]; then
      if [[ "$item" == *set_proxy.sh ]]; then
        continue
      fi
      search_and_replace "$item" "$new_proxy"
    fi
  done
}

check_proxy_format() {
  local proxy_string="$1"
  local pattern="https_proxy=http://[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:[0-9]+"
  if [[ -z "$proxy_string" ]]; then
    return
  fi
  if [[ ! "$proxy_string" =~ $pattern ]]; then
    echo proxy format error, need like this: https_proxy=http://ip:port
    exit 1
  fi
}

check_proxy_format "$new_proxy_value"
recursive_search "$scripts_path" "$new_proxy_value"
