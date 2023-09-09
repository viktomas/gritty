#!/bin/bash

# Get the number of lines and columns in the terminal
rows=$(tput lines)
cols=$(tput cols)

# Fill the screen
for ((i = 0; i < $rows; i++)); do
  for ((j = 0; j < $cols; j++)); do # 13 is the length of "Hello, World!"
    echo -n "H"
  done
  echo # Move to the next line
done
