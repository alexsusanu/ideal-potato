#include "vehicleDefinition.hpp"
#include <iostream>

int main() {
    std::cout << "Creating a motorised vehicle..." << std::endl;
    Motorised ford("Ford", "petrol");
    std::cout << "Created a motorised vehicle: " << ford.make << std::endl;

    std::cout << "----------------------------------------" << std::endl;

    std::cout << "Creating an aircraft..." << std::endl;
    Aircraft boeing("Boeing", 3, "kerosene");
    std::cout << "Created an aircraft: " << boeing.make << std::endl;

    std::cout << "----------------------------------------" << std::endl;

    boeing.switchOn();
    std::cout << "" << std::endl;
    boeing.takeOff();

    return 0;
}