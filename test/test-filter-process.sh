#!/usr/bin/env bash

. "test/testlib.sh"

# HACK(taylor): git uses ".g<hash>" in the version name to signal that it is
# from the "next" branch, which is the only (current) version of Git that has
# support for the filter protocol.
#
# Once 2.11 is released, replace this with:
#
# ```
# ensure_git_version_isnt $VERSION_LOWER "2.11.0"
# ```
if [ "1" -ne "$(git version | cut -d ' ' -f 3 | grep -c "g")" ]; then
  echo "skip: $0 git version does not include support for filter protocol"
  exit
fi

begin_test "filter process: checking out a branch"
(
  set -e

  reponame="filter_process_checkout"
  setup_remote_repo "$reponame"
  clone_repo "$reponame" repo

  git lfs track "*.dat"
  git add .gitattributes
  git commit -m "initial commit"

  contents_a="contents_a"
  contents_a_oid="$(calc_oid $contents_a)"
  printf "$contents_a" > a.dat

  git add a.dat
  git commit -m "add a.dat"

  git checkout -b b

  contents_b="contents_b"
  contents_b_oid="$(calc_oid $contents_b)"
  printf "$contents_b" > b.dat

  git add b.dat
  git commit -m "add b.dat"

  git push origin --all

  pushd ..
    # Git will choose filter.lfs.process over `filter.lfs.clean` and
    # `filter.lfs.smudge`
    git \
      -c "filter.lfs.process=git-lfs filter-process" \
      -c "filter.lfs.clean=false"\
      -c "filter.lfs.smudge=false" \
      -c "filter.lfs.required=true" \
      clone "$GITSERVER/$reponame" "$reponame-assert"

    cd "$reponame-assert"

    # Assert that we are on the "master" branch, and have a.dat
    [ "master" = "$(git rev-parse --abbrev-ref HEAD)" ]
    [ "$contents_a" = "$(cat a.dat)" ]
    assert_pointer "master" "a.dat" "$contents_a_oid" 10

    git checkout b

    # Assert that we are on the "b" branch, and have b.dat
    [ "b" = "$(git rev-parse --abbrev-ref HEAD)" ]
    [ "$contents_b" = "$(cat b.dat)" ]
    assert_pointer "b" "b.dat" "$contents_b_oid" 10
  popd
)
end_test

begin_test "filter process: adding a file"
(
  set -e

  reponame="filter_process_add"
  setup_remote_repo "$reponame"
  clone_repo "$reponame" "$reponame"

  git lfs track "*.dat"
  git add .gitattributes
  git commit -m "initial commit"

  contents="contents"
  contents_oid="$(calc_oid "$contents")"
  printf "$contents" > a.dat

  git add a.dat

  expected="$(pointer "$contents_oid" "${#contents}")"
  got="$(git cat-file -p :a.dat)"

  diff -u <(echo "$expected") <(echo "$got")
)
end_test
