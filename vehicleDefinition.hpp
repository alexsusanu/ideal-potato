#ifndef vehicleDefinition
#define vehicleDefinition

#include <string>
#include <iostream>

class Vehicle {
public:
    std::string make;
public:
    Vehicle(const std::string& make) : make(make) {}
};

class Wheeled : virtual public Vehicle {
protected:
    int wheels;
public:
    Wheeled(const std::string& make, int wheels) : Vehicle(make), wheels(wheels) {}
    int getWheels() const { return wheels; }
};

class Motorised : virtual public Vehicle {
protected:
    std::string typeOfEngine;
public:
    Motorised(const std::string& make, const std::string& typeOfEngine) : Vehicle(make), typeOfEngine(typeOfEngine) {
        std::cout << "Wroom!" << std::endl;
    }
    void switchOn() const {
        std::cout << "The " << typeOfEngine << " engine is now on." << std::endl;
    }
};

class Aircraft : public Wheeled, public Motorised {
public:
    Aircraft(const std::string& make, int wheels, const std::string& typeOfEngine) : Vehicle(make), Wheeled(make, wheels), Motorised(make, typeOfEngine) {}
    
    void takeOff() const {
        std::cout << "Aircraft preparing to take off" << std::endl;
        std::cout << "Make: " << make << std::endl;
        std::cout << "Wheels: " << wheels << std::endl;
        std::cout << "Engine type: " << typeOfEngine << std::endl;
    }
};

#endif