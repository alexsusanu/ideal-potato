import time

class PIDController:

    # PID controller class
    # Kp, Ki, Kd - proportional, integral, derivative
    # setpoint - target pressure to maintain
    # previous_error is the error from the previous run, initialised to 0
    # integral is the integral of the error, initialised to 0
    # derivative is the derivative of the error
    # output is the output of the controller    
    def __init__(self, Kp, Ki, Kd, setpoint):
        self.Kp = Kp
        self.Ki = Ki
        self.Kd = Kd
        self.setpoint = setpoint
        self.previous_error = 0
        self.integral = 0

    def compute(self, current_pressure):

        # error is the difference between the desired pressure and the current pressure
        error = self.setpoint - current_pressure

        # integral - accumulated error over time
        self.integral += error
        derivative = error - self.previous_error

        # output - PID formula
        output = (self.Kp * error) + (self.Ki * self.integral) + (self.Kd * derivative)

        # update the previous error to be the current error
        self.previous_error = error

        # output will be anything between 0 and 100
        return max(0, min(100, output))  

class Aperture:

    # start the aperture at 50% 
    def __init__(self):
        self.position = 50
        self.target_position = 50

    def set_target(self, target):
        self.target_position = max(0, min(100, target))

    # increase or decrease by 0.5
    def update(self):
        if abs(self.target_position - self.position) > 1:
            self.position += 0.5 if self.target_position > self.position else -0.5
        return self.position

def simulate_pressure(aperture_position):
    base_pressure = 100 - aperture_position
    # water fluctuation
    fluctuation = (hash(time.time()) % 20) - 10 
    return max(0, base_pressure + fluctuation)

def main():
    setpoint = 50
    pid = PIDController(Kp=0.5, Ki=0.01, Kd=0.1, setpoint=setpoint)
    aperture = Aperture()

    for _ in range(100):
        current_pressure = simulate_pressure(aperture.position)
        aperture_adjustment = pid.compute(current_pressure)
        aperture.set_target(aperture_adjustment)
        aperture_position = aperture.update()

        print(f"Pressure: {current_pressure:.2f}, Aperture: {aperture_position:.2f}%")
        time.sleep(0.1)

if __name__ == "__main__":
    main()