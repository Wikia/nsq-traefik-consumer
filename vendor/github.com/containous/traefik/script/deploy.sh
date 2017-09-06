#!/usr/bin/env bash
set -e

if [ -n "$TRAVIS_TAG" ]; then
  echo "Deploying..."
else
  echo "Skipping deploy"
  exit 0
fi

git config --global user.email "$TRAEFIKER_EMAIL"
git config --global user.name "Traefiker"

# load ssh key
echo "Loading key..."
openssl aes-256-cbc -K $encrypted_f9e835a425bc_key -iv $encrypted_f9e835a425bc_iv -in .travis/traefiker_rsa.enc -out ~/.ssh/traefiker_rsa -d
eval "$(ssh-agent -s)"
chmod 600 ~/.ssh/traefiker_rsa
ssh-add ~/.ssh/traefiker_rsa

# update traefik-library-image repo (official Docker image)
echo "Updating traefik-library-imag repo..."
git clone git@github.com:containous/traefik-library-image.git
cd traefik-library-image
./update.sh $VERSION
git add -A
echo $VERSION | git commit --file -
echo $VERSION | git tag -a $VERSION --file -
git push -q --follow-tags -u origin master > /dev/null 2>&1

# create docker image emilevauge/traefik (compatibility)
echo "Updating docker emilevauge/traefik image..."
docker login -u $DOCKER_USER -p $DOCKER_PASS
docker tag containous/traefik emilevauge/traefik:latest
docker push emilevauge/traefik:latest
docker tag emilevauge/traefik:latest emilevauge/traefik:${VERSION}
docker push emilevauge/traefik:${VERSION}

cd ..
rm -Rf traefik-library-image/

echo "Deployed"