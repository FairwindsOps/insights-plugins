for d in ./plugins/*/ ; do
    echo "$d"
    if ! git diff --exit-code --quiet $d; then
      version=$(cat $d/version.txt | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
      echo $version > $d/version.txt
    fi
done

