#include "math/add.h"
#include "math/mul.h"

#include <iostream>

int main(int argc, char** argv) {
  std::cout << "3 times 5 is: " << math::MultiplyInt(3, 5) << std::endl;
  std::cout << "3 plus 5 is: " << math::AddInt(3, 5) << std::endl;

  return 0;
}
