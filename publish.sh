tag=$1
note=$2
gh release create $tag --title "" --notes "$note" --draft
gh release upload $tag /Users/aramneja/GoProjects/gotdr/bins/gotdr_*
gh release edit $tag --draft=false