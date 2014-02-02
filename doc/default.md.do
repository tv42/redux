# combine various bits to create a single documentation file

redo-ifchange $2.txt

if test $2 = "redo" ; then
  name=$2
else
  name="redo-"$2
fi

cat <<EOS
% $name(1) Redux User Manual 
% Gyepi Sam
% $(date +'%B %d, %Y') 

<!-- DO NOT EDIT -- Autogenerated file. Really! -->

EOS

../bin/redux documentation $2 | sed 's/^\( \+-.\+\)/\1\n/'

echo

cat $2.txt
